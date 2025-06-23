package events

import (
	"context"
	"log/slog"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
)

func (h *Handler) handleCodeScanningAlertEvent(_ context.Context, log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	if team.SlackChannels.Security == "" {
		return nil, nil
	}

	if event.Alert.Rule.SeverityType() < team.Config.Security.SeverityType() {
		return nil, nil
	}

	log.Info("Received code scanning alert")
	return slack.CreateCodeScanningAlertMessage(team.SlackChannels.Security, event), nil
}
