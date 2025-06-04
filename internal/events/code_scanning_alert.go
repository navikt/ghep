package events

import (
	"context"
	"log/slog"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
)

func (h *Handler) handleCodeScanningAlertEvent(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	if team.SlackChannels.Security == "" {
		return nil, nil
	}

	return slack.CreateCodeScanningAlertMessage(team.SlackChannels.Security, event), nil
}
