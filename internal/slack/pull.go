package slack

import (
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
				Text:  attachmentText,
				Color: color,
			},
		},
	}
}
