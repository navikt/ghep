package slack

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/navikt/ghep/internal/github"
)

func CreateTeamMessage(tmpl template.Template, channel string, event github.Event) ([]byte, error) {
	type text struct {
		Channel    string
		Repository github.Repository
		Team       github.TeamEvent
		Action     string
	}
	payload := text{
		Action:     "added to",
		Channel:    channel,
		Repository: event.Repository,
		Team:       *event.Team,
	}

	if event.Action == "removed_from_repository" {
		payload.Action = "removed from"
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, payload); err != nil {
		return nil, fmt.Errorf("error executing template: %w", err)
	}

	return output.Bytes(), nil
}
