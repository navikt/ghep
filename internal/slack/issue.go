package slack

import (
	"encoding/json"
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateIssueMessage(channel, threadTimestamp string, event github.Event) *Message {
	color := "#34a44c"

	text := fmt.Sprintf("Issue <%s|#%d> %s in `<%s|%s>` by <%s|%s>.", event.Issue.URL, event.Issue.Number, event.Action, event.Repository.URL, event.Repository.Name, event.Sender.URL, event.Sender.Login)

	if event.Action == "closed" {
		color = "#7044c4"
		text = fmt.Sprintf("Issue <%s|#%d> %s as %s in `<%s|%s>` by <%s|%s>.", event.Issue.URL, event.Issue.Number, event.Action, event.Issue.StateReason, event.Repository.URL, event.Repository.Name, event.Sender.URL, event.Sender.Login)
	}

	attachmentText := fmt.Sprintf("*<%s|#%d %s>*", event.Issue.URL, event.Issue.Number, event.Issue.Title)

	if event.Action == "opened" {
		attachmentText = fmt.Sprintf("*<%s|#%d %s>*\n%s", event.Issue.URL, event.Issue.Number, event.Issue.Title, event.Issue.Body)
	}

	return &Message{
		Channel:         channel,
		ThreadTimestamp: threadTimestamp,
		Text:            text,
		Attachments: []Attachment{
			{
				Text:       attachmentText,
				Color:      color,
				FooterIcon: netrualGithubIcon,
				Footer:     fmt.Sprintf("<%s|%s>", event.Repository.URL, event.Repository.FullName),
			},
		},
	}
}

func (c Client) PostUpdatedIssueMessage(msg, action, timestamp string) error {
	var message Message
	if err := json.Unmarshal([]byte(msg), &message); err != nil {
		return fmt.Errorf("unmarshalling message: %w", err)
	}

	if action != "closed" {
		return nil
	}

	message.Timestamp = timestamp
	message.Attachments[0].Color = "#7044c4"

	marshalled, err := json.Marshal(message)
	if err != nil {
		return err
	}

	c.log.Info("Posting update of issue", "action", action, "channel", message.Channel, "timestamp", timestamp)
	_, err = c.postRequest("chat.update", marshalled)
	if err != nil {
		return err
	}

	return nil
}
