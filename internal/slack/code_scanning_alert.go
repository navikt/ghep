package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateCodeScanningAlertMessage(channel string, event github.Event) *Message {
	var attachments []Attachment
	if event.Action == "created" {
		color := getColorBySeverity(event.Alert.Rule.SeverityType())

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
