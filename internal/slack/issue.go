package slack

import (
	"fmt"
	"html"
	"strings"

	"github.com/navikt/ghep/internal/github"
)

func CreateIssueMessage(channel, threadTimestamp string, event github.Event) *Message {
	color := ColorOpened

	text := fmt.Sprintf("Issue <%s|#%d> %s in `%s` by %s", event.Issue.URL, event.Issue.Number, event.Action, event.Repository.ToSlack(), event.Sender.ToSlack())
	attachmentText := fmt.Sprintf("*<%s|#%d %s>*\n%s", event.Issue.URL, event.Issue.Number, html.EscapeString(event.Issue.Title), event.Issue.Body)

	if event.Action == "closed" {
		color = ColorMerged
		text = fmt.Sprintf("Issue <%s|#%d> %s as %s in `%s` by %s", event.Issue.URL, event.Issue.Number, event.Action, event.Issue.StateReason, event.Repository.ToSlack(), event.Sender.ToSlack())
		attachmentText = fmt.Sprintf("*<%s|#%d %s>*", event.Issue.URL, event.Issue.Number, html.EscapeString(event.Issue.Title))
	}

	return &Message{
		Channel:         channel,
		ThreadTimestamp: threadTimestamp,
		Text:            text,
		Attachments: []Attachment{
			{
				Text:       attachmentText,
				Type:       "mrkdwn",
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
		color = ColorOpened
	case "closed":
		color = ColorMerged
	default:
		text = fmt.Sprintf("Issue <%s|#%d> %s in `%s` by %s", event.Issue.URL, event.Issue.Number, event.Action, event.Repository.ToSlack(), event.Sender.ToSlack())
		attachmentText = fmt.Sprintf("*<%s|#%d %s>*\n%s", event.Issue.URL, event.Issue.Number, html.EscapeString(event.Issue.Title), event.Issue.Body)

		if event.Issue.State == "closed" {
			color = ColorMerged
		}
	}

	if len(event.Issue.Assignees) > 0 {
		var assignees strings.Builder
		for i, assignee := range event.Issue.Assignees {
			fmt.Fprintf(&assignees, "@%s", assignee.Login)

			if i < len(event.Issue.Assignees)-1 {
				assignees.WriteString(", ")
			}
		}

		attachmentText += fmt.Sprintf("\n*Assignees:* %s", assignees.String())
	}

	message.Text = text
	message.Attachments[0].Color = color
	message.Attachments[0].Text = attachmentText

	return &message
}
