package slack

import (
	"encoding/json"
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateIssueMessage(channel, threadTimestamp string, event github.Event) *Message {
	color := "#34a44c"

	text := fmt.Sprintf("Issue <%s|#%d> %s in `<%s|%s>` by <%s|%s>.", event.Issue.URL, event.Issue.Number, event.Action, event.Repository.URL, event.Repository.Name, event.Sender.URL, event.Sender.Login)
	attachmentText := fmt.Sprintf("*<%s|#%d %s>*\n%s", event.Issue.URL, event.Issue.Number, event.Issue.Title, event.Issue.Body)

	if event.Action == "closed" {
		color = "#7044c4"
		text = fmt.Sprintf("Issue <%s|#%d> %s as %s in `<%s|%s>` by <%s|%s>.", event.Issue.URL, event.Issue.Number, event.Action, event.Issue.StateReason, event.Repository.URL, event.Repository.Name, event.Sender.URL, event.Sender.Login)
		attachmentText = fmt.Sprintf("*<%s|#%d %s>*", event.Issue.URL, event.Issue.Number, event.Issue.Title)
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

func (c Client) PostUpdatedIssueMessage(msg, timestamp string, event github.Event) error {
	var message Message
	if err := json.Unmarshal([]byte(msg), &message); err != nil {
		return fmt.Errorf("unmarshalling message: %w", err)
	}

	color := message.Attachments[0].Color
	text := message.Text
	attachmentText := message.Attachments[0].Text

	switch event.Action {
	case "reopened":
		color = "#34a44c"
	case "closed":
		color = "#7044c4"
	case "edited":
		text = fmt.Sprintf("Issue <%s|#%d> %s in `<%s|%s>` by <%s|%s>.", event.Issue.URL, event.Issue.Number, event.Action, event.Repository.URL, event.Repository.Name, event.Sender.URL, event.Sender.Login)
		attachmentText = fmt.Sprintf("*<%s|#%d %s>*\n%s", event.Issue.URL, event.Issue.Number, event.Issue.Title, event.Issue.Body)
	}

	message.Timestamp = timestamp
	message.Text = text
	message.Attachments[0].Color = color
	message.Attachments[0].Text = attachmentText

	marshalled, err := json.Marshal(message)
	if err != nil {
		return err
	}

	c.log.Info("Posting update of issue", "channel", message.Channel, "timestamp", timestamp)
	_, err = c.postRequest("chat.update", marshalled)
	if err != nil {
		return err
	}

	return nil
}
