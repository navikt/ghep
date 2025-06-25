package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateSecurityAdvisoryMessage(channel string, event github.Event) *Message {
	text := fmt.Sprintf("A security advisory was just %s for the repository %s.", event.Action, event.Repository.ToSlack())
	attachments := []Attachment{
		{
			Text:       fmt.Sprintf("*%s*\n%s", event.SecurityAdvisory.Summary, event.SecurityAdvisory.Description),
			Color:      getColorBySeverity(event.Alert.SecurityAdvisory.SeverityType()),
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
