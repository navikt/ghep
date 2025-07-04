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
	"github.com/redis/go-redis/v9"
)

func (h *Handler) handlePullRequestEvent(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	id := strconv.Itoa(event.PullRequest.ID)
	timestamp, err := h.redis.Get(ctx, id).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		log.Error("error getting thread timestamp", "err", err.Error(), "id", id)
	}

	if !slices.Contains([]string{"opened", "closed", "reopened", "edited", "review_requested", "review_request_removed"}, event.Action) {
		return nil, nil
	}

	messageBytes, err := h.redis.Get(ctx, timestamp).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		log.Error("error getting message", "err", err.Error(), "timestamp", timestamp)
	}

	if !errors.Is(err, redis.Nil) && event.Action != "opened" {
		var oldMessage slack.Message
		if err := json.Unmarshal([]byte(messageBytes), &oldMessage); err != nil {
			log.Error("error unmarshalling message", "err", err.Error())
		}

		updatedMessage := slack.CreateUpdatedPullRequestMessage(oldMessage, event)
		updatedMessage.Timestamp = timestamp

		marshalled, err := json.Marshal(updatedMessage)
		if err != nil {
			log.Error("error marshalling message", "err", err.Error())
		}

		log.Info("Posting update of pull request", "channel", updatedMessage.Channel, "timestamp", timestamp)
		_, err = h.slack.PostUpdatedMessage(marshalled)
		if err != nil {
			log.Error("error posting updated message", "err", err.Error())
		}

		if slices.Contains([]string{"reopened", "edited"}, event.Action) {
			return nil, nil
		}
	}

	channel := team.SlackChannels.PullRequests
	if _, err := h.db.GetUser(ctx, event.Sender.Login); err != nil {
		if err != pgx.ErrNoRows {
			channel = team.Config.ExternalContributorsChannel
		}
	}

	return handlePullRequestEvent(log, team, timestamp, channel, event)
}

func handlePullRequestEvent(log *slog.Logger, team github.Team, threadTimestamp, channel string, event github.Event) (*slack.Message, error) {
	if !slices.Contains([]string{"opened", "closed", "reopened"}, event.Action) {
		return nil, nil
	}

	if channel == "" {
		return nil, nil
	}

	log.Info("Received pull request", "slack_channel", channel)
	return slack.CreatePullRequestMessage(channel, threadTimestamp, event), nil
}
