package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreatePublicizedMessage(channel string, event github.Event) *Message {
	return &Message{
		Channel: channel,
		Text:    fmt.Sprintf("<%s|%s> made <%s|%s> public!", event.Sender.URL, event.Sender.Login, event.Repository.URL, event.Repository.Name),
	}
}
