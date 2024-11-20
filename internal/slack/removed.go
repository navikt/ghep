package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateRemovedMessage(channel string, event github.Event) *Message {
	return &Message{
		Channel: channel,
		Text:    fmt.Sprintf("<%s|%s> deleted the repository `%s`.", event.Sender.URL, event.Sender.Login, event.RepositoriesRemoved[0].Name),
	}
}
