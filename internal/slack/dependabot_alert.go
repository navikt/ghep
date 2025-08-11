package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateDependabotAlertMessage(channel, timestamp string, event github.Event) *Message {
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
		Channel:         channel,
		Text:            fmt.Sprintf("A Dependabot alert was just %s for the repository %s.\nRead more: %s", event.Action, event.Repository.ToSlack(), event.Alert.URL),
		ThreadTimestamp: timestamp, // TODO: Må bruke chat.update for å oppdatere meldingen
		Attachments:     attachments,
	}
}
