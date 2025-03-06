package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateTeamMessage(channel string, event github.Event) *Message {
	var text string

	switch event.Action {
	case "added_to_repository":
		text = fmt.Sprintf("Team %s was added to the repository %s", event.Team.ToSlack(), event.Repository.ToSlack())
	case "removed_from_repository":
		text = fmt.Sprintf("Team %s was removed from the repository %s", event.Team.ToSlack(), event.Repository.ToSlack())
	case "added":
		text = fmt.Sprintf("%s was added to the team %s", event.Member.ToSlack(), event.Team.ToSlack())
	case "removed":
		text = fmt.Sprintf("%s was removed from the team %s", event.Member.ToSlack(), event.Team.ToSlack())
	}

	return &Message{
		Channel: channel,
		Text:    text,
	}
}
