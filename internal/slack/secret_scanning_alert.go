package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateSecretScanningAlertMessage(channel string, event github.Event) *Message {
	var text string
	switch event.Action {
	case "created":
		text = fmt.Sprintf("A secret scanning alert was just %s for the repository %s.\nThe secret was of type `%s`.\nRead more: %s", event.Action, event.Repository.ToSlack(), *event.Alert.SecretType, event.Alert.URL)
	case "resolved":
		text = fmt.Sprintf("A secret scanning alert was just %s for the repository %s. It was resolved as `%s` by %s.\n> %s", event.Action, event.Repository.ToSlack(), event.Alert.Resolution, event.Alert.ResolvedBy.ToSlack(), event.Alert.ResolutionComment)
	}

	var attachments []Attachment
	if event.Alert.PubliclyLeaked && event.Action == "created" {
		attachment := Attachment{
			Text:  "The secret was publicly leaked!",
			Color: ColorCritical,
		}
		attachments = append(attachments, attachment)
	}

	return &Message{
		Channel:     channel,
		Text:        text,
		Attachments: attachments,
	}
}
