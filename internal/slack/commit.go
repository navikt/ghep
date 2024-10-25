package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
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

func fetchCoAuthors(log *slog.Logger, githubClient github.Userer, commit github.Commit) ([]github.User, error) {
	coAuthorsRegexp := regexp.MustCompile(`Co-authored-by: (.*) <(.*)>`)

	var coAuthors []github.User
	coAuthorsMatches := coAuthorsRegexp.FindAllStringSubmatch(commit.Message, -1)

	for _, match := range coAuthorsMatches {
		name := match[1]
		email := ""
		if len(match) > 2 {
			email = match[2]
		}

		user := github.User{
			Name:  name,
			Login: name,
		}

		if strings.HasPrefix(name, "@") {
			// Prefix with @ to indicate that this is a GitHub user
			after, _ := strings.CutPrefix(match[1], "@")
			user.Login = after
			user.URL = "https://github.com/" + after
		} else if strings.HasSuffix(email, "@users.noreply.github.com") {
			// If the email is a GitHub noreply email, we can extract the username from it
			before, _ := strings.CutSuffix(email, "@users.noreply.github.com")
			_, after, found := strings.Cut(before, "+")
			if found {
				user.Login = after
				user.URL = "https://github.com/" + after
			}
		} else if strings.HasSuffix(email, "@nav.no") {
			// If the email is a NAV email, we can look up the username
			userWithEmail, err := githubClient.GetUserByEmail(email)
			if err != nil {
				log.Error("Failed to get user by email", "email", email, "error", err)
			}

			user = userWithEmail
		}

		coAuthors = append(coAuthors, user)
	}

	return coAuthors, nil
}

func createAuthors(log *slog.Logger, githubClient github.Userer, event github.Event, team github.Team) (string, error) {
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

		coAuthors, err := fetchCoAuthors(log, githubClient, commit)
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

func CreateCommitMessage(log *slog.Logger, tmpl template.Template, channel string, event github.Event, team github.Team, githubClient github.Userer) ([]byte, error) {
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

	authors, err := createAuthors(log, githubClient, event, team)
	if err != nil {
		return nil, fmt.Errorf("creating authors: %w", err)
	}

	payload.Senders = authors

	var output bytes.Buffer
	if err := tmpl.Execute(&output, payload); err != nil {
		return nil, fmt.Errorf("executing commit template: %w", err)
	}

	return output.Bytes(), nil
}
