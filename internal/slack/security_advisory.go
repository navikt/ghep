package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateSecurityAdvisoryMessage(channel string, event github.Event) *Message {
	text := fmt.Sprintf("A security advisory was just %s for the repository %s.", event.Action, event.Repository.ToSlack())

	color := ColorDefault
	switch event.SecurityAdvisory.Severity {
	case "critical":
		color = ColorCritical
	case "high":
		color = ColorHigh
	case "moderate":
		color = ColorMedium
	case "low":
		color = ColorLow
	default:
		color = ColorDefault
	}

	attachments := []Attachment{
		{
			Text:       fmt.Sprintf("*%s*\n%s", event.SecurityAdvisory.Summary, event.SecurityAdvisory.Description),
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
