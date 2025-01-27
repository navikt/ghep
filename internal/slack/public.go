package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreatePublicizedMessage(channel string, event github.Event) *Message {
	return &Message{
		Channel: channel,
		Text:    fmt.Sprintf("%s made %s public!", event.Sender.ToSlack(), event.Repository.ToSlack()),
	}
}
