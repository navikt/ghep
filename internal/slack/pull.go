package slack

import (
	"fmt"
	"html"

	"github.com/navikt/ghep/internal/github"
)

func CreatePullRequestMessage(channel, threadTimestamp string, event github.Event) *Message {
	color := ColorOpened

	if event.PullRequest.Merged {
		event.Action = "merged"
		color = ColorMerged
	}

	if event.Action == "closed" {
		color = ColorClosed
	}

	var text string
	eventType := "Pull request"
	if event.PullRequest.Draft {
		eventType = "Draft pull request"
		color = ColorDraft
	}

	text = fmt.Sprintf("%s <%s|#%d> %s in `%s` by %s", eventType, event.PullRequest.URL, event.PullRequest.Number, event.Action, event.Repository.ToSlack(), event.Sender.ToSlack())
	if event.Action == "closed" {
		text = fmt.Sprintf("%s <%s|#%d> %s as %s in `%s` by %s", eventType, event.PullRequest.URL, event.PullRequest.Number, event.Action, event.PullRequest.State, event.Repository.ToSlack(), event.Sender.ToSlack())
	}

	attachmentText := fmt.Sprintf("*<%s|#%d %s>*", event.PullRequest.URL, event.PullRequest.Number, html.EscapeString(event.PullRequest.Title))

	if event.Action == "opened" {
		attachmentText = fmt.Sprintf("*<%s|#%d %s>*\n%s", event.PullRequest.URL, event.PullRequest.Number, html.EscapeString(event.PullRequest.Title), event.PullRequest.Body)
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

func CreateUpdatedPullRequestMessage(message Message, event github.Event) *Message {
	color := message.Attachments[0].Color
	text := message.Text
	attachmentText := message.Attachments[0].Text

	switch event.Action {
	case "reopened":
		color = ColorOpened
	case "closed":
		if event.PullRequest.Merged {
			color = ColorMerged
		} else {
			color = ColorClosed
		}
	case "edited":
		eventType := "Pull request"
		if event.PullRequest.Draft {
			eventType = "Draft pull request"
			color = ColorDraft
		}

		text = fmt.Sprintf("%s <%s|#%d> %s in `%s` by %s", eventType, event.PullRequest.URL, event.PullRequest.Number, event.Action, event.Repository.ToSlack(), event.Sender.ToSlack())
		attachmentText = fmt.Sprintf("*<%s|#%d %s>*\n%s", event.PullRequest.URL, event.PullRequest.Number, html.EscapeString(event.PullRequest.Title), event.PullRequest.Body)

		if event.PullRequest.Merged {
			color = ColorMerged
		} else if event.PullRequest.State == "closed" {
			color = ColorClosed
		}
	}

	message.Text = text
	message.Attachments[0].Color = color
	message.Attachments[0].Text = attachmentText

	return &message
}
