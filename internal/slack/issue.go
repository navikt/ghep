package slack

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/navikt/ghep/internal/github"
)

func CreateIssueMessage(tmpl template.Template, channel string, event github.Event) ([]byte, error) {
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
		Number:     event.Issue.Number,
		Sender:     event.Sender,
		Status:     event.Issue.StateReason,
	}

	payload.Attachment.Title = event.Issue.Title
	payload.Attachment.Body = event.Issue.Body
	payload.Attachment.URL = event.Issue.URL

	var output bytes.Buffer
	if err := tmpl.Execute(&output, payload); err != nil {
		return nil, fmt.Errorf("executing commit template: %w", err)
	}

	return output.Bytes(), nil
}
