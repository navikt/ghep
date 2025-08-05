package slack

import (
	"fmt"
	"html"
	"strings"

	"github.com/navikt/ghep/internal/github"
)

func CreatePullRequestMessage(channel, threadTimestamp string, event github.Event) *Message {
	color := ColorOpened
	if event.Action == "closed" {
		color = ColorClosed
		if event.PullRequest.Merged {
			color = ColorMerged
			event.Action = "merged"
		}
	}

	eventType := "Pull request"
	if event.PullRequest.Draft {
		eventType = "Draft pull request"
		color = ColorDraft
	}

	text := fmt.Sprintf("%s <%s|#%d> %s in `%s` by %s", eventType, event.PullRequest.URL, event.PullRequest.Number, event.Action, event.Repository.ToSlack(), event.Sender.ToSlack())
	attachmentText := fmt.Sprintf("*<%s|#%d %s>*", event.PullRequest.URL, event.PullRequest.Number, html.EscapeString(event.PullRequest.Title))

	if event.Action != "closed" && event.PullRequest.Body != "" {
		attachmentText = fmt.Sprintf("%s\n%s", attachmentText, event.PullRequest.Body)
	}

	if len(event.PullRequest.RequestedReviewers) > 0 {
		var reviewers strings.Builder
		for i, reviewer := range event.PullRequest.RequestedReviewers {
			fmt.Fprintf(&reviewers, "@%s", reviewer.Login)

			if i < len(event.PullRequest.RequestedReviewers)-1 {
				reviewers.WriteString(", ")
			}
		}

		attachmentText += fmt.Sprintf("\n*Requested reviewers:* %s", reviewers.String())
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
