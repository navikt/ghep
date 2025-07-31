package events

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strconv"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/redis/go-redis/v9"
)

func (h *Handler) handleReleaseEvent(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	var timestamp string
	var err error
	if event.Action == "edited" {
		id := strconv.Itoa(event.Release.ID)
		timestamp, err = h.redis.Get(ctx, id).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			log.Error("error getting thread timestamp", "err", err.Error(), "id", id)
		}

		updatedMessage := slack.CreateReleaseMessage(team.SlackChannels.Releases, event)
		updatedMessage.Timestamp = timestamp

		log.Info("Posting update of release", "channel", updatedMessage.Channel, "timestamp", updatedMessage.Timestamp)
		if err = h.slack.PostUpdatedMessage(*updatedMessage); err != nil {
			log.Error("error posting updated message", "err", err.Error(), "channel", updatedMessage.Channel, "timestamp", timestamp)
		}

		return nil, nil
	}

	return handleReleaseEvent(log, team, event)
}

func handleReleaseEvent(log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	if !slices.Contains([]string{"published"}, event.Action) {
		return nil, nil
	}

	if team.SlackChannels.Releases == "" {
		return nil, nil
	}

	log.Info("Received release", "slack_channel", team.SlackChannels.Releases)
	return slack.CreateReleaseMessage(team.SlackChannels.Releases, event), nil
}
