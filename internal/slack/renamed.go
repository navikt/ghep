package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateRenamedMessage(channel string, event github.Event) *Message {
	return &Message{
		Channel: channel,
		Text:    fmt.Sprintf("<%s|%s> renamed the repository `%s` to <%s|%s>.", event.Sender.URL, event.Sender.Login, event.Changes.Repository.Name.From, event.Repository.URL, event.Repository.Name),
	}
}
