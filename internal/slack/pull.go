package slack

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/navikt/ghep/internal/github"
)

func CreatePullRequestMessage(tmpl template.Template, channel string, event github.Event) ([]byte, error) {
	type text struct {
		Channel    string
		Repository github.Repository
		Action     string
		Number     int
		Sender     github.Sender
		Status     string
		Attachment struct {
			Title string
			Body  string
			URL   string
		}
	}
	payload := text{
		Channel:    channel,
		Repository: event.Repository,
		Action:     event.Action,
		Number:     event.PullRequest.Number,
		Sender:     event.Sender,
	}

	payload.Attachment.Title = event.PullRequest.Title
	payload.Attachment.Body = event.PullRequest.Body
	payload.Attachment.URL = event.PullRequest.URL

	if event.PullRequest.Merged {
		payload.Action = "merged"
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, payload); err != nil {
		return nil, fmt.Errorf("executing commit template: %w", err)
	}

	return output.Bytes(), nil
}
