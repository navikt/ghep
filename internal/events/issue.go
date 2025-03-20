package events

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strconv"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/redis/go-redis/v9"
)

func (h *Handler) handleIssueEvent(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	id := strconv.Itoa(event.Issue.ID)
	timestamp, err := h.redis.Get(ctx, id).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		log.Error("error getting thread timestamp", "err", err.Error(), "id", id)
	}

	if !slices.Contains([]string{"opened", "closed", "reopened", "edited"}, event.Action) {
		log.Info("unknown issue action")
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
		marshalled, err := json.Marshal(updatedMessage)
		if err != nil {
			log.Error("error marshalling message", "err", err.Error())
		}

		log.Info("Posting update of issue", "channel", updatedMessage.Channel, "timestamp", timestamp)
		_, err = h.slack.PostUpdatedMessage(marshalled)
		if err != nil {
			log.Error("error posting updated message", "err", err.Error(), "channel", updatedMessage.Channel, "timestamp", timestamp)
		}

		if slices.Contains([]string{"reopened", "edited"}, event.Action) {
			return nil, nil
		}
	}

	return handleIssueEvent(log, team, timestamp, event)
}

func handleIssueEvent(log *slog.Logger, team github.Team, threadTimestamp string, event github.Event) (*slack.Message, error) {
	if !slices.Contains([]string{"opened", "closed"}, event.Action) {
		return nil, nil
	}

	channel := team.SlackChannels.Issues
	if team.IsExternalContributor(event.Sender) {
		channel = team.Config.ExternalContributorsChannel
	}

	if channel == "" {
		return nil, nil
	}

	log.Info("Received issue", "slack_channel", channel)
	return slack.CreateIssueMessage(channel, threadTimestamp, event), nil
}
