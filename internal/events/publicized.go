package events

import (
	"log/slog"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
)

func handlePublicizedEvent(log *slog.Logger, channel string, event github.Event) *slack.Message {
	log.Info("Received repository publicized", "channel", channel)
	return slack.CreateRenamedMessage(channel, event)
}
