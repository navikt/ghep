package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/navikt/ghep/internal/github"
)

func CreateIssueMessage(tmpl template.Template, channel, threadTimestamp string, event github.Event) ([]byte, error) {
	type text struct {
		Channel         string
		ThreadTimestamp string
		Repository      github.Repository
		Action          string
		Number          int
		Sender          github.Sender
		Status          string
		Color           string
		Attachment      struct {
			Title string
			Body  string
			URL   string
		}
	}
	payload := text{
		Channel:         channel,
		ThreadTimestamp: threadTimestamp,
		Repository:      event.Repository,
		Action:          event.Action,
		Number:          event.Issue.Number,
		Sender:          event.Sender,
		Status:          event.Issue.StateReason,
		Color:           "#34a44c",
	}

	marshaledTitle, err := json.Marshal(event.Issue.Title)
	if err != nil {
		return nil, fmt.Errorf("marshalling issue: %w", err)
	}
	title := string(marshaledTitle)
	title = title[1 : len(title)-1]

	marshaledBody, err := json.Marshal(event.Issue.Body)
	if err != nil {
		return nil, fmt.Errorf("marshalling issue: %w", err)
	}
	body := string(marshaledBody)
	body = body[1 : len(body)-1]

	payload.Attachment.Title = title
	payload.Attachment.Body = body
	payload.Attachment.URL = event.Issue.URL

	if event.Action == "closed" {
		payload.Color = "#d02434"
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, payload); err != nil {
		return nil, fmt.Errorf("executing commit template: %w", err)
	}

	return output.Bytes(), nil
}
