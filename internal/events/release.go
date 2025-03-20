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
	}

	return handleReleaseEvent(log, team, event, timestamp)
}

func handleReleaseEvent(log *slog.Logger, team github.Team, event github.Event, timestamp string) (*slack.Message, error) {
	if !slices.Contains([]string{"published", "edited"}, event.Action) {
		return nil, nil
	}

	if team.SlackChannels.Releases == "" {
		return nil, nil
	}

	log.Info("Received release", "slack_channel", team.SlackChannels.Releases)
	return slack.CreateReleaseMessage(team.SlackChannels.Releases, timestamp, event), nil
}
