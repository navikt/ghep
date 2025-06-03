package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateDependabotAlertMessage(channel string, event github.Event) *Message {
	return &Message{
		Channel: channel,
		Text:    fmt.Sprintf("A Dependabot alert was just %s (%s) for the repository %s.\nRead more: %s", event.Action, event.DependabotAlert.State, event.Repository.ToSlack(), event.DependabotAlert.URL),
	}
}
