package events

import (
	"log/slog"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
)

func handleRenamedEvent(log *slog.Logger, channel string, event github.Event) *slack.Message {
	log.Info("Posting renamed repository message", "channel", channel)
	return slack.CreateRenamedMessage(channel, event)
}
