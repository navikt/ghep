package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateSecurityAdvisoryMessage(channel string, event github.Event) *Message {
	securityAdvisoryURL := event.SecurityAdvisory.References[len(event.SecurityAdvisory.References)-1].URL
	attachments := []Attachment{
		{
			Text:       fmt.Sprintf("*%s*\n%s", event.SecurityAdvisory.Summary, event.SecurityAdvisory.Description),
			Color:      getColorBySeverity(event.SecurityAdvisory.SeverityType()),
			FooterIcon: neutralGithubIcon,
			Footer:     fmt.Sprintf("<%s|Github Advisories>", securityAdvisoryURL),
		},
	}

	return &Message{
		Channel:     channel,
		Text:        fmt.Sprintf("A <%s|security advisory> was just %s.", securityAdvisoryURL, event.Action),
		Attachments: attachments,
	}
}
