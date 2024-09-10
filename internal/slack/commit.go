package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
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

func createAuthors(event github.Event) (string, error) {
	authors := []github.User{event.Sender}

	for _, commit := range event.Commits {
		if commit.Author.Username == "" {
			continue
		}

		containsCompare := func(user github.User) bool {
			return user.Login == commit.Author.Username
		}

		if slices.ContainsFunc(authors, containsCompare) {
			continue
		}

		author := github.User{
			Login: commit.Author.Username,
			URL:   "https://github.com/" + commit.Author.Username,
		}

		authors = append(authors, author)
	}

	authorsAsString := make([]string, len(authors))
	for i, author := range authors {
		authorsAsString[i] = fmt.Sprintf("<%s|%s>", author.URL, author.Login)
	}

	var senders string
	if len(authorsAsString) == 1 {
		senders = authorsAsString[0]
	} else {
		senders = strings.Join(authorsAsString[0:len(authorsAsString)-1], ", ")
		senders += ", and " + authorsAsString[len(authorsAsString)-1]
	}

	return senders, nil
}

func CreateCommitMessage(tmpl template.Template, channel string, event github.Event) ([]byte, error) {
	type text struct {
		Channel         string
		URL             string
		Repository      string
		Senders         string
		NumberOfCommits int
		AttachmentsText string
		Compare         string
	}

	payload := text{
		Channel:         channel,
		URL:             event.Repository.URL,
		Repository:      event.Repository.Name,
		NumberOfCommits: len(event.Commits),
		Compare:         event.Compare,
	}

	attachmentsText, err := createAttachmentsText(event.Commits)
	if err != nil {
		return nil, fmt.Errorf("creating attachments text: %w", err)
	}

	payload.AttachmentsText = attachmentsText

	authors, err := createAuthors(event)
	payload.Senders = authors

	var output bytes.Buffer
	if err := tmpl.Execute(&output, payload); err != nil {
		return nil, fmt.Errorf("executing commit template: %w", err)
	}

	return output.Bytes(), nil
}
