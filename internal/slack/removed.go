package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateRemovedMessage(channel string, event github.Event) *Message {
	return &Message{
		Channel: channel,
		Text:    fmt.Sprintf("%s deleted the repository `%s`.", event.Sender.ToSlack(), event.RepositoriesRemoved[0].Name),
	}
}
