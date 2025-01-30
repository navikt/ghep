package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateIssueMessage(channel, threadTimestamp string, event github.Event) *Message {
	color := "#34a44c"

	text := fmt.Sprintf("Issue <%s|#%d> %s in `%s` by %s.", event.Issue.URL, event.Issue.Number, event.Action, event.Repository.ToSlack(), event.Sender.ToSlack())
	attachmentText := fmt.Sprintf("*<%s|#%d %s>*\n%s", event.Issue.URL, event.Issue.Number, event.Issue.Title, event.Issue.Body)

	if event.Action == "closed" {
		color = "#7044c4"
		text = fmt.Sprintf("Issue <%s|#%d> %s as %s in `%s` by %s.", event.Issue.URL, event.Issue.Number, event.Action, event.Issue.StateReason, event.Repository.ToSlack(), event.Sender.ToSlack())
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
				FooterIcon: neutralGithubIcon,
				Footer:     fmt.Sprintf("<%s|%s>", event.Repository.URL, event.Repository.FullName),
			},
		},
	}
}

func CreateUpdatedIssueMessage(message Message, event github.Event) *Message {
	color := message.Attachments[0].Color
	text := message.Text
	attachmentText := message.Attachments[0].Text

	switch event.Action {
	case "reopened":
		color = "#34a44c"
	case "closed":
		color = "#7044c4"
	case "edited":
		text = fmt.Sprintf("Issue <%s|#%d> %s in `%s` by %s.", event.Issue.URL, event.Issue.Number, event.Action, event.Repository.ToSlack(), event.Sender.ToSlack())
		attachmentText = fmt.Sprintf("*<%s|#%d %s>*\n%s", event.Issue.URL, event.Issue.Number, event.Issue.Title, event.Issue.Body)

		if event.Issue.State == "closed" {
			color = "#7044c4"
		}
	}

	message.Text = text
	message.Attachments[0].Color = color
	message.Attachments[0].Text = attachmentText

	return &message
}
