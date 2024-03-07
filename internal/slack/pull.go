package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/navikt/ghep/internal/github"
)

func CreatePullRequestMessage(tmpl template.Template, channel, threadTimestamp string, event github.Event) ([]byte, error) {
	type text struct {
		Channel         string
		ThreadTimestamp string
		Repository      github.Repository
		Action          string
		Draft           bool
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
		Draft:           event.PullRequest.Draft,
		Number:          event.PullRequest.Number,
		Sender:          event.Sender,
		Color:           "#34a44c",
	}

	marshaledTitle, err := json.Marshal(event.PullRequest.Title)
	if err != nil {
		return nil, fmt.Errorf("marshalling pull request: %w", err)
	}
	title := string(marshaledTitle)
	title = title[1 : len(title)-1]

	marshaledBody, err := json.Marshal(event.PullRequest.Body)
	if err != nil {
		return nil, fmt.Errorf("marshalling pull request: %w", err)
	}
	body := string(marshaledBody)
	body = body[1 : len(body)-1]

	payload.Attachment.Title = title
	payload.Attachment.Body = body
	payload.Attachment.URL = event.PullRequest.URL

	if event.PullRequest.Merged {
		payload.Action = "merged"
		payload.Color = "#7044c4"
	}

	if payload.Action == "closed" && !event.PullRequest.Merged {
		payload.Color = "#d02434"
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, payload); err != nil {
		return nil, fmt.Errorf("executing commit template: %w", err)
	}

	return output.Bytes(), nil
}
