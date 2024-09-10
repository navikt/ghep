package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/navikt/ghep/internal/github"
)

func createAttachmentsText(commits []github.Commit) (string, error) {
	var attachementText strings.Builder
	for _, c := range commits {
		firstLine := strings.Split(c.Message, "\n")[0]

		attachementText.WriteString(fmt.Sprintf("`<%s|%s>` - %s\n", c.URL, c.ID[:8], firstLine))
	}

	var marshalled bytes.Buffer
	enc := json.NewEncoder(&marshalled)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(attachementText.String()); err != nil {
		return "", fmt.Errorf("marshalling commit messages: %w", err)
	}

	return strings.TrimSuffix(marshalled.String(), "\n"), nil
}

func CreateCommitMessage(tmpl template.Template, channel string, event github.Event) ([]byte, error) {
	type text struct {
		Channel         string
		URL             string
		Repository      string
		Sender          github.User
		NumberOfCommits int
		AttachmentsText string
		Compare         string
	}

	payload := text{
		Channel:         channel,
		URL:             event.Repository.URL,
		Sender:          event.Sender,
		Repository:      event.Repository.Name,
		NumberOfCommits: len(event.Commits),
		Compare:         event.Compare,
	}

	attachmentsText, err := createAttachmentsText(event.Commits)
	if err != nil {
		return nil, fmt.Errorf("creating attachments text: %w", err)
	}

	payload.AttachmentsText = attachmentsText

	payload.AttachmentsText = strings.TrimSuffix(marshalled.String(), "\n")

	var output bytes.Buffer
	if err := tmpl.Execute(&output, payload); err != nil {
		return nil, fmt.Errorf("executing commit template: %w", err)
	}

	return output.Bytes(), nil
}
