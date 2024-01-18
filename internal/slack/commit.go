package slack

import (
	"bytes"
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateCommitMessage(channel string, event github.CommitEvent) ([]byte, error) {
	type commit struct {
		URL     string
		Ref     string
		Message string
	}

	type text struct {
		Channel    string
		URL        string
		Repository string
		Author     string
		Commits    []commit
	}

	payload := text{
		Channel:    channel,
		URL:        event.Repository.HtmlUrl,
		Author:     event.Commits[0].Author.Name,
		Repository: event.Repository.Name,
	}

	commits := []commit{}
	for _, c := range event.Commits {
		commits = append(commits, commit{
			Ref:     c.Id[:8],
			Message: c.Message,
			URL:     c.Url,
		})
	}
	payload.Commits = commits

	var output bytes.Buffer
	if err := commitTmpl.Execute(&output, payload); err != nil {
		return nil, fmt.Errorf("executing commit template: %w", err)
	}

	return output.Bytes(), nil
}
