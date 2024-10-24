package slack

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/navikt/ghep/internal/github"
)

func CreateRemovedMessage(tmpl template.Template, channel string, event github.Event) ([]byte, error) {
	type text struct {
		Channel    string
		Repository string
		Sender     github.User
	}
	payload := text{
		Channel:    channel,
		Repository: event.RepositoriesRemoved[0].Name,
		Sender:     event.Sender,
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, payload); err != nil {
		return nil, fmt.Errorf("error executing template: %w", err)
	}

	return output.Bytes(), nil
}
