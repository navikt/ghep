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

func (h *Handler) handlePullRequestEvent(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	var timestamp string
	if slices.Contains([]string{"closed", "reopened", "edited", "review_requested", "review_request_removed"}, event.Action) {

		id := strconv.Itoa(event.PullRequest.ID)
		message, err := h.db.GetSlackMessage(ctx, gensql.GetSlackMessageParams{
			TeamSlug: team.Name,
			EventID:  id,
		})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			log.Error("error getting thread timestamp", "err", err.Error(), "id", id)
		}

		if message.ThreadTs != "" {
			timestamp = message.ThreadTs

			if message.Payload != nil && event.Action != "edited" {
				var oldMessage slack.Message
				if err := json.Unmarshal(message.Payload, &oldMessage); err != nil {
					log.Error("error unmarshalling message", "err", err.Error())
				}

				updatedMessage := slack.CreatePullRequestMessage(ctx, log, h.db, oldMessage.Channel, timestamp, team.Config.PingSlackUsers, event)
				updatedMessage.Timestamp = timestamp

				log.Info("Posting update of pull request", "channel", updatedMessage.Channel, "timestamp", updatedMessage.Timestamp)
				if err = h.slack.PostUpdatedMessage(*updatedMessage); err != nil {
					log.Error("error posting updated message", "err", err.Error())
				}

				return nil, nil
			}
		}
	}

	return handlePullRequestEvent(ctx, log, h.db, team, timestamp, event)
}

func handlePullRequestEvent(ctx context.Context, log *slog.Logger, db sql.Userer, team github.Team, threadTimestamp string, event github.Event) (*slack.Message, error) {
	if team.Config.Pulls.IgnoreBots && event.Sender.IsBot() {
		return nil, nil
	}

	if !slices.Contains([]string{"opened", "closed"}, event.Action) {
		return nil, nil
	}

	if team.SlackChannels.PullRequests == "" {
		return nil, nil
	}

	channel := team.SlackChannels.PullRequests
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
				log.Error("error getting team member", "err", err.Error(), "user", event.Sender.Login)
			}
		}
	}

	log.Info("Received pull request", "slack_channel", channel)
	return slack.CreatePullRequestMessage(ctx, log, db, channel, threadTimestamp, team.Config.PingSlackUsers, event), nil
}
