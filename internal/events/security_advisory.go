package events

import (
	"context"
	"log/slog"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
)

func (h *Handler) handleSecurityAdvisoryEvent(_ context.Context, log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	if team.SlackChannels.Security == "" {
		return nil, nil
	}

	if event.SecurityAdvisory.SeverityType() < team.Config.Security.SeverityType() {
		return nil, nil
	}

	log.Info("Received security advisory", "severity", event.SecurityAdvisory.Severity)
	return slack.CreateSecurityAdvisoryMessage(team.SlackChannels.Security, event), nil
}
