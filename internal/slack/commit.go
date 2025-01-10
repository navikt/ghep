package slack

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"

	"github.com/navikt/ghep/internal/github"
)

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

			if userWithEmail != nil {
				user = *userWithEmail
			}
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

func CreateCommitMessage(log *slog.Logger, channel string, event github.Event, team github.Team, githubClient github.Userer) (*Message, error) {
	authors, err := createAuthors(log, githubClient, event, team)
	if err != nil {
		return nil, fmt.Errorf("creating authors: %w", err)
	}

	text := fmt.Sprintf("<%s|%d new commits> pushed to `<%s|%s>` by %s.", event.Compare, len(event.Commits), event.Repository.URL, event.Repository.Name, authors)

	var attachmentText strings.Builder
	for _, c := range event.Commits {
		firstLine := strings.Split(c.Message, "\n")[0]

		attachmentText.WriteString(fmt.Sprintf("`<%s|%s>` - %s\n", c.URL, c.ID[:8], firstLine))
	}

	attachments := []Attachment{
		{
			Text:  attachmentText.String(),
			Color: "#000",
		},
	}

	return &Message{
		Channel:     channel,
		Text:        text,
		Attachments: attachments,
	}, nil
}

func (c Client) PostUpdatedCommitMessage(log *slog.Logger, msg string, event github.Event, timestamp string) error {
	var message Message
	if err := json.Unmarshal([]byte(msg), &message); err != nil {
		return fmt.Errorf("unmarshalling message: %w", err)
	}

	if message.Attachments[0].Footer != "" {
		return nil
	}

	message.Timestamp = timestamp
	message.Attachments[0].FooterIcon = netrualGithubIcon
	message.Attachments[0].Footer = fmt.Sprintf("<%s|%s>", event.Workflow.URL, event.Workflow.Name)

	marshalled, err := json.Marshal(message)
	if err != nil {
		return err
	}

	log.Info("Posting update of commit", "channel", message.Channel, "timestamp", timestamp)
	_, err = c.postRequest("chat.update", marshalled)
	if err != nil {
		return err
	}

	return nil
}
