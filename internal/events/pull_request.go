package events

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/navikt/ghep/internal/sql"
	"github.com/navikt/ghep/internal/sql/gensql"
)

func (h *Handler) handlePullRequestEvent(ctx context.Context, log *slog.Logger, team github.Team, source github.Source, event github.Event) (*slack.Message, error) {
	var timestamp string
	if !slices.Contains([]string{"opened", "synchronize"}, event.Action) {

		id := strconv.Itoa(event.PullRequest.ID)
		message, err := h.db.GetSlackMessage(ctx, gensql.GetSlackMessageParams{
			TeamSlug: team.Name,
			EventID:  id,
			Channel:  source.Channel,
		})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			log.Error("Getting thread timestamp", "error", err, "id", id)
		}

		if message.ThreadTs != "" {
			timestamp = message.ThreadTs

			if message.Payload != nil && event.Action != "edited" {
				var oldMessage slack.Message
				if err := json.Unmarshal(message.Payload, &oldMessage); err != nil {
					log.Error("Unmarshalling message", "error", err)
				}

				updatedMessage := slack.CreatePullRequestMessage(ctx, log, h.db, oldMessage.Channel, timestamp, team.Config.PingSlackUsers, source.Config.Pulls.Minimalist, event)
				updatedMessage.Timestamp = timestamp

				log.Info("Posting update of pull request", "channel", updatedMessage.Channel, "timestamp", updatedMessage.Timestamp)
				if err = h.slack.PostUpdatedMessage(*updatedMessage); err != nil {
					log.Error("Posting updated message", "error", err)
				}

				return nil, nil
			}
		}
	}

	return handlePullRequestEvent(ctx, log, h.db, team, source, timestamp, event)
}

func handlePullRequestEvent(ctx context.Context, log *slog.Logger, db sql.Userer, team github.Team, source github.Source, threadTimestamp string, event github.Event) (*slack.Message, error) {
	prIsFromBot := event.Sender.IsBot() || event.PullRequest.User.IsBot()

	if source.Config.Pulls.IgnoreBots && prIsFromBot {
		return nil, nil
	}

	if source.Config.Pulls.OnlyBots && !prIsFromBot {
		return nil, nil
	}

	if !slices.Contains([]string{"opened", "closed", "ready_for_review"}, event.Action) {
		return nil, nil
	}

	if source.Config.Pulls.IgnoreDrafts && event.PullRequest.Draft {
		return nil, nil
	}

	if len(source.Config.Pulls.Events) > 0 {
		action := event.Action
		if action == "closed" && event.PullRequest.Merged {
			action = "merged"
		}

		if !slices.Contains(source.Config.Pulls.Events, action) {
			return nil, nil
		}
	}

	channel := source.Channel
	if event.Sender.IsUser() {
		if _, err := db.GetTeamMember(ctx, gensql.GetTeamMemberParams{
			TeamSlug:  team.Name,
			UserLogin: event.Sender.Login,
		}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				if team.Config.ExternalContributorsChannel != "" {
					channel = team.Config.ExternalContributorsChannel
				}
			} else {
				log.Error("Getting team member", "error", err, "user", event.Sender.Login)
			}
		}
	}

	log.Info("Received pull request", "channel", channel)
	return slack.CreatePullRequestMessage(ctx, log, db, channel, threadTimestamp, team.Config.PingSlackUsers, source.Config.Pulls.Minimalist, event), nil
}
