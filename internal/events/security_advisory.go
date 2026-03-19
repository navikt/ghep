package events

import (
	"context"
	"log/slog"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
)

func (h *Handler) handleSecurityAdvisoryEvent(_ context.Context, log *slog.Logger, team github.Team, source github.Source, event github.Event) (*slack.Message, error) {
	if event.SecurityAdvisory.SeverityType() < source.Config.Security.SeverityType() {
		return nil, nil
	}

	log.Info("Received security advisory", "severity", event.SecurityAdvisory.Severity)
	return slack.CreateSecurityAdvisoryMessage(source.Channel, event), nil
}
