package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
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

func fetchCoAuthors(commit github.Commit) ([]github.User, error) {
	coAuthorsRegexp := regexp.MustCompile(`Co-authored-by: (.*) <.*>`)

	var coAuthors []github.User
	coAuthorsMatches := coAuthorsRegexp.FindAllStringSubmatch(commit.Message, -1)

	for _, match := range coAuthorsMatches {
		name := match[1]

		user := github.User{
			Name:  name,
			Login: name,
		}

		if strings.HasPrefix(name, "@") {
			after, _ := strings.CutPrefix(match[1], "@")
			user.Login = after
			user.URL = "https://github.com/" + after
		}

		coAuthors = append(coAuthors, user)
	}

	return coAuthors, nil
}

func createAuthors(event github.Event, team github.Team) (string, error) {
	compareUsernameFunc := func(username string) func(github.User) bool {
		return func(user github.User) bool {
			return user.Login == username
		}
	}

	compareNameFunc := func(name string) func(github.User) bool {
		return func(user github.User) bool {
			return user.Name == name
		}
	}

	// sender has login/username and url
	// co-authors only have username or name
	// author has name, email, and username
	authors := []github.User{event.Sender}

	for _, commit := range event.Commits {
		if slices.ContainsFunc(authors, compareUsernameFunc(commit.Author.Username)) {
			i := slices.IndexFunc(authors, compareUsernameFunc(commit.Author.Username))
			author := authors[i]
			author.Name = commit.Author.Name
			authors[i] = author
		} else {
			authors = append(authors, commit.Author.AsUser())
		}

		coAuthors, err := fetchCoAuthors(commit)
		if err != nil {
			// TODO: Log error, but continue
			return "", fmt.Errorf("fetching co-authors: %w", err)
		}

		for _, coAuthor := range coAuthors {
			if slices.ContainsFunc(authors, compareUsernameFunc(coAuthor.Login)) {
				continue
			}

			if slices.ContainsFunc(authors, compareNameFunc(coAuthor.Name)) {
				continue
			}

			if member, ok := team.GetMemberByName(coAuthor.Login); ok {
				authors = append(authors, member)
				continue
			}

			authors = append(authors, coAuthor)
		}
	}

	authorsAsString := make([]string, len(authors))
	for i, author := range authors {
		authorsAsString[i] = author.ToSlack()
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

func CreateCommitMessage(tmpl template.Template, channel string, event github.Event, team github.Team) ([]byte, error) {
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

	authors, err := createAuthors(event, team)
	payload.Senders = authors

	var output bytes.Buffer
	if err := tmpl.Execute(&output, payload); err != nil {
		return nil, fmt.Errorf("executing commit template: %w", err)
	}

	return output.Bytes(), nil
}
