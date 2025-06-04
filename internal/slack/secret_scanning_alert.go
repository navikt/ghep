package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateSecretScanningAlertMessage(channel string, event github.Event) *Message {
	var attachments []Attachment
	if event.Alert.PubliclyLeaked {
		attachment := Attachment{
			Text:  "The secret was publicly leaked!",
			Color: "#ff0000",
		}
		attachments = append(attachments, attachment)
	}

	return &Message{
		Channel:     channel,
		Text:        fmt.Sprintf("A secret scanning alert was just %s (%s) for the repository %s.\nThe secret was of type %s.\nRead more: %s", event.Action, event.Alert.State, *event.Alert.SecretType, event.Repository.ToSlack(), event.Alert.URL),
		Attachments: attachments,
	}
}
