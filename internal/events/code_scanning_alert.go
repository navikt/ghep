package events

import (
	"context"
	"log/slog"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/navikt/ghep/internal/sql/gensql"
)

func (h *Handler) handleCodeScanningAlertEvent(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	if team.SlackChannels.Security == "" {
		return nil, nil
	}

	if event.Action == "appeared_in_branch" {
		return nil, nil
	}

	if event.Alert.Rule.SeverityType() < team.Config.Security.SeverityType() {
		return nil, nil
	}

	message, err := h.db.GetSlackMessage(ctx, gensql.GetSlackMessageParams{
		TeamSlug: team.Name,
		EventID:  event.Alert.URL,
	})
	if err != nil {
		log.Error("error getting slack message", "err", err.Error(), "event_id", event.Alert.URL)
	}

	channel := team.SlackChannels.Security
	if message.Channel != "" {
		channel = message.Channel
	}

	log.Info("Received code scanning alert")
	return slack.CreateCodeScanningAlertMessage(channel, message.ThreadTs, event), nil
}
