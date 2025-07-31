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
	"github.com/redis/go-redis/v9"
)

func (h *Handler) handleIssueEvent(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	id := strconv.Itoa(event.Issue.ID)
	timestamp, err := h.redis.Get(ctx, id).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		log.Error("error getting thread timestamp", "err", err.Error(), "id", id)
	}

	if !slices.Contains([]string{"opened", "closed", "reopened", "edited", "assigned", "unassigned"}, event.Action) {
		return nil, nil
	}

	msgBytes, err := h.redis.Get(ctx, timestamp).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		log.Error("error getting message", "err", err.Error(), "timestamp", timestamp)
	}

	if !errors.Is(err, redis.Nil) && event.Action != "opened" {
		var oldMessage slack.Message
		if err := json.Unmarshal([]byte(msgBytes), &oldMessage); err != nil {
			log.Error("error unmarshalling message", "err", err.Error())
		}

		updatedMessage := slack.CreateUpdatedIssueMessage(oldMessage, event)
		updatedMessage.Timestamp = timestamp

		log.Info("Posting update of issue", "channel", updatedMessage.Channel, "timestamp", updatedMessage.Timestamp)
		if err = h.slack.PostUpdatedMessage(*updatedMessage); err != nil {
			log.Error("error posting updated message", "err", err.Error(), "channel", updatedMessage.Channel, "timestamp", timestamp)
		}

		if slices.Contains([]string{"reopened", "edited"}, event.Action) {
			return nil, nil
		}
	}

	return handleIssueEvent(ctx, log, h.db, team, timestamp, event)
}

func handleIssueEvent(ctx context.Context, log *slog.Logger, db sql.TeamQuery, team github.Team, threadTimestamp string, event github.Event) (*slack.Message, error) {
	if !slices.Contains([]string{"opened", "closed"}, event.Action) {
		return nil, nil
	}
	if team.SlackChannels.PullRequests == "" {
		return nil, nil
	}

	channel := team.SlackChannels.Issues
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

	log.Info("Received issue", "slack_channel", channel)
	return slack.CreateIssueMessage(channel, threadTimestamp, event), nil
}
