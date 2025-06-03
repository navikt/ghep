package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateSecurityAdvisoryMessage(channel string, event github.Event) *Message {
	text := fmt.Sprintf("A security advisory (%s) was just %s for the repository %s", event.SecurityAdvisory.State, event.Action, event.Repository.ToSlack())

	color := "#00ffff"
	switch event.SecurityAdvisory.Severity {
	case "critical":
		color = "#ff0000"
	case "high":
		color = "#ff8000"
	case "medium":
		color = "#ffff00"
	case "low":
		color = "#00ff00"
	}

	attachments := []Attachment{
		{
			Text:       event.SecurityAdvisory.Summary,
			Color:      color,
			FooterIcon: neutralGithubIcon,
			Footer:     fmt.Sprintf("<%s|%s>", event.SecurityAdvisory.URL, event.SecurityAdvisory.CVEID),
		},
	}

	return &Message{
		Channel:     channel,
		Text:        text,
		Attachments: attachments,
	}
}
