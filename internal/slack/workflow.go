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
		Sender     github.User
		Status     string
		Workflow   *github.Workflow
		FailedJob  github.FailedJob
	}
	payload := text{
		Channel:    channel,
		Repository: event.Repository,
		Sender:     event.Sender,
		Workflow:   event.Workflow,
		FailedJob:  event.Workflow.FailedJob,
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, payload); err != nil {
		return nil, fmt.Errorf("executing commit template: %w", err)
	}

	return output.Bytes(), nil
}
