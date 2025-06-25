package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateDependabotAlertMessage(channel string, event github.Event, timestamp string) *Message {
	var attachments []Attachment
	if event.Action == "created" {
		attachments = []Attachment{
			{
				Text:  event.Alert.SecurityAdvisory.Summary,
				Color: ColorDefault,
			},
		}
	}

	return &Message{
		Channel:     channel,
		Text:        fmt.Sprintf("A Dependabot alert was just %s for the repository %s.\nRead more: %s", event.Action, event.Repository.ToSlack(), event.Alert.URL),
		Timestamp:   timestamp,
		Attachments: attachments,
	}
}
