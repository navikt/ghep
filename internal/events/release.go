package events

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/navikt/ghep/internal/sql/gensql"
)

func (h *Handler) handleReleaseEvent(ctx context.Context, log *slog.Logger, team github.Team, source github.Source, event github.Event) (*slack.Message, error) {
	if event.Action == "edited" {
		id := strconv.Itoa(event.Release.ID)
		message, err := h.db.GetSlackMessage(ctx, gensql.GetSlackMessageParams{
			TeamSlug: team.Name,
			EventID:  id,
			Channel:  source.Channel,
		})
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				log.Error("error getting thread timestamp", "err", err.Error(), "id", id)
			}

			return nil, nil
		}

		updatedMessage := slack.CreateReleaseMessage(source.Channel, event)
		updatedMessage.Timestamp = message.ThreadTs

		log.Info("Posting update of release", "channel", updatedMessage.Channel, "timestamp", updatedMessage.Timestamp)
		if err = h.slack.PostUpdatedMessage(*updatedMessage); err != nil {
			log.Error("error posting updated message", "err", err.Error(), "channel", updatedMessage.Channel, "timestamp", updatedMessage.Timestamp)
		}

		return nil, nil
	}

	return handleReleaseEvent(log, source, event)
}

func handleReleaseEvent(log *slog.Logger, source github.Source, event github.Event) (*slack.Message, error) {
	if !slices.Contains([]string{"published"}, event.Action) {
		return nil, nil
	}

	log.Info("Received release", "slack_channel", source.Channel)
	return slack.CreateReleaseMessage(source.Channel, event), nil
}
