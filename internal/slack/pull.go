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
		Number:          event.PullRequest.Number,
		Sender:          event.Sender,
		Color:           "#36A750",
	}

	marshaledText, err := json.Marshal(event.PullRequest.Body)
	if err != nil {
		return nil, fmt.Errorf("marshalling pull request: %w", err)
	}
	marshaledText = bytes.Trim(marshaledText, "\"")

	payload.Attachment.Title = event.PullRequest.Title
	payload.Attachment.Body = string(marshaledText)
	payload.Attachment.URL = event.PullRequest.URL

	if event.PullRequest.Merged {
		payload.Action = "merged"
		payload.Color = "#8251df"
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, payload); err != nil {
		return nil, fmt.Errorf("executing commit template: %w", err)
	}

	return output.Bytes(), nil
}
