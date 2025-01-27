package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateTeamMessage(channel string, event github.Event) *Message {
	action := "added to"
	if event.Action == "removed_from_repository" {
		action = "removed from"
	}

	return &Message{
		Channel: channel,
		Text:    fmt.Sprintf("Team <%s|%s> was %s the repository %s.", event.Team.URL, event.Team.Name, action, event.Repository.ToSlack()),
	}
}
