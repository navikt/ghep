package slack

import (
	"encoding/json"
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreatePullRequestMessage(channel, threadTimestamp string, event github.Event) *Message {
	action := event.Action
	color := "#34a44c"

	if event.PullRequest.Merged {
		action = "merged"
		color = "#7044c4"
	}

	if event.Action == "closed" && !event.PullRequest.Merged {
		color = "#d02434"
	}

	var text string
	if event.PullRequest.Draft {
		text = fmt.Sprintf("Draft pull request <%s|#%d> %s in `<%s|%s>` by <%s|%s>.", event.PullRequest.URL, event.PullRequest.Number, action, event.Repository.URL, event.Repository.Name, event.Sender.URL, event.Sender.Login)
	} else {
		text = fmt.Sprintf("Pull request <%s|#%d> %s in `<%s|%s>` by <%s|%s>.", event.PullRequest.URL, event.PullRequest.Number, action, event.Repository.URL, event.Repository.Name, event.Sender.URL, event.Sender.Login)
	}

	attachmentText := fmt.Sprintf("*<%s|#%d %s>*", event.PullRequest.URL, event.PullRequest.Number, event.PullRequest.Title)

	if event.Action == "opened" {
		attachmentText = fmt.Sprintf("*<%s|#%d %s>*\n%s", event.PullRequest.URL, event.PullRequest.Number, event.PullRequest.Title, event.PullRequest.Body)
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

func (c Client) PostUpdatedPullMessage(msg string, event github.Event, timestamp string) error {
	var message Message
	if err := json.Unmarshal([]byte(msg), &message); err != nil {
		return fmt.Errorf("unmarshalling message: %w", err)
	}

	color := message.Attachments[0].Color

	if event.PullRequest.Merged {
		color = "#7044c4"
	}

	if event.Action == "closed" && !event.PullRequest.Merged {
		color = "#d02434"
	}

	message.Timestamp = timestamp
	message.Attachments[0].Color = color

	marshalled, err := json.Marshal(message)
	if err != nil {
		return err
	}

	c.log.Info("Posting update of pull", "action", event.Action, "channel", message.Channel, "timestamp", timestamp)
	_, err = c.postRequest("chat.update", marshalled)
	if err != nil {
		return err
	}

	return nil
}
