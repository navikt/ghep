package slack

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/navikt/ghep/internal/github"
)

func CreateRenamedMessage(tmpl template.Template, channel string, event github.Event) ([]byte, error) {
	type text struct {
		Channel string
		From    string
		To      struct {
			Name string
			URL  string
		}
		Sender github.User
	}
	payload := text{
		Channel: channel,
		From:    event.Changes.Repository.Name.From,
		To: struct {
			Name string
			URL  string
		}{
			Name: event.Repository.Name,
			URL:  event.Repository.URL,
		},
		Sender: event.Sender,
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, payload); err != nil {
		return nil, fmt.Errorf("error executing template: %w", err)
	}

	return output.Bytes(), nil
}
