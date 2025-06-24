package events

import (
	"context"
	"log/slog"
	"slices"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
)

func (h *Handler) handleSecretScanningAlertEvent(_ context.Context, log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	if team.SlackChannels.Security == "" {
		return nil, nil
	}

	if !slices.Contains([]string{"created", "fixed"}, event.Action) {
		return nil, nil
	}

	log.Info("Received secret scanning alert", "secret_type", event.Alert.SecretType)
	return slack.CreateSecretScanningAlertMessage(team.SlackChannels.Security, event), nil
}
