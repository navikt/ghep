package slack

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/navikt/ghep/internal/github"
)

func CreateWorkflowMessage(tmpl template.Template, channel string, event github.Event) ([]byte, error) {
	type text struct {
		Channel    string
		Repository github.Repository
		Sender     github.Sender
		Status     string
		Workflow   *github.Workflow
	}
	payload := text{
		Channel:    channel,
		Repository: event.Repository,
		Sender:     event.Sender,
		Workflow:   event.Workflow,
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, payload); err != nil {
		return nil, fmt.Errorf("executing commit template: %w", err)
	}

	return output.Bytes(), nil
}
