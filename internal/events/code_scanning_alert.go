package events

import (
	"context"
	"log/slog"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/navikt/ghep/internal/sql/gensql"
)

func (h *Handler) handleCodeScanningAlertEvent(ctx context.Context, log *slog.Logger, team github.Team, source github.Source, event github.Event) (*slack.Message, error) {
	if event.Action == "appeared_in_branch" {
		return nil, nil
	}

	if event.Alert.Rule.SeverityType() < source.Config.Security.SeverityType() {
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

	channel := source.Channel
	if message.Channel != "" {
		channel = message.Channel
	}

	log.Info("Received code scanning alert")
	return slack.CreateCodeScanningAlertMessage(channel, message.ThreadTs, event), nil
}
