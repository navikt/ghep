package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateCodeScanningAlertMessage(channel string, event github.Event) *Message {
	var attachments []Attachment
	if event.Action == "created" {
		color := ColorDefault
		switch event.Alert.Rule.Severity() {
		case github.SeverityCritical:
			color = ColorCritical
		case github.SeverityHigh:
			color = ColorHigh
		case github.SeverityMedium:
			color = ColorMedium
		case github.SeverityLow:
			color = ColorLow
		}

		attachments = []Attachment{
			{
				Text:  fmt.Sprintf("*%s*\n%s", event.Alert.Rule.Description, event.Alert.Rule.FullDescription),
				Color: color,
			},
		}
	}

	return &Message{
		Channel:     channel,
		Text:        fmt.Sprintf("A code scanning alert was just %s for the repository %s.\nRead more: %s", event.Action, event.Repository.ToSlack(), event.Alert.URL),
		Attachments: attachments,
	}
}
