package slack

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/navikt/ghep/internal/github"
)

func CreateCommitMessage(tmpl *template.Template, channel string, event github.Event) ([]byte, error) {
	type commit struct {
		URL     string
		Ref     string
		Message string
	}

	type text struct {
		Channel    string
		URL        string
		Repository string
		Sender     github.Sender
		Commits    []commit
		Compare    string
	}

	payload := text{
		Channel:    channel,
		URL:        event.Repository.URL,
		Sender:     event.Sender,
		Repository: event.Repository.Name,
		Compare:    event.Compare,
	}

	commits := []commit{}
	for _, c := range event.Commits {
		message := strings.Split(c.Message, "\n")[0]

		commits = append(commits, commit{
			Ref:     c.ID[:8],
			Message: message,
			URL:     c.URL,
		})
	}
	payload.Commits = commits

	var output bytes.Buffer
	if err := tmpl.Execute(&output, payload); err != nil {
		return nil, fmt.Errorf("executing commit template: %w", err)
	}

	return output.Bytes(), nil
}
