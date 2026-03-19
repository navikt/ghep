package events

import (
	"context"
	"log/slog"
	"slices"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/navikt/ghep/internal/sql/gensql"
)

func (h *Handler) handleSecretScanningAlertEvent(ctx context.Context, log *slog.Logger, team github.Team, source github.Source, event github.Event) (*slack.Message, error) {
	if !slices.Contains([]string{"created", "fixed", "resolved"}, event.Action) {
		return nil, nil
	}

	message, err := h.db.GetSlackMessage(ctx, gensql.GetSlackMessageParams{
		TeamSlug: team.Name,
		EventID:  event.Alert.URL,
		Channel:  source.Channel,
	})
	if err != nil {
		log.Error("Getting slack message", "error", err, "event_id", event.Alert.URL)
	}

	log.Info("Received secret scanning alert", "secret_type", event.Alert.SecretType)
	return slack.CreateSecretScanningAlertMessage(source.Channel, message.ThreadTs, event), nil
}
