package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateRenamedMessage(channel string, event github.Event) *Message {
	return &Message{
		Channel: channel,
		Text:    fmt.Sprintf("%s renamed the repository `%s` to %s", event.Sender.ToSlack(), event.Changes.Repository.Name.From, event.Repository.ToSlack()),
	}
}
