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

func fetchCoAuthors(commit string) ([]github.Author, error) {
	coAuthorsRegexp := regexp.MustCompile(`Co-authored-by: (.*) <(.*)>`)

	var coAuthors []github.Author
	coAuthorsMatches := coAuthorsRegexp.FindAllStringSubmatch(commit, -1)

	for _, match := range coAuthorsMatches {
		name := match[1]
		email := ""
		if len(match) > 2 {
			email = match[2]
		}

		author := github.Author{
			Name:  name,
			Email: email,
		}

		if strings.HasPrefix(name, "@") {
			// Prefix with @ to indicate that this is a GitHub user
			after, _ := strings.CutPrefix(match[1], "@")
			author.Username = after
		} else if strings.HasSuffix(email, "@users.noreply.github.com") {
			// If the email is a GitHub noreply email, we can extract the username from it
			before, _ := strings.CutSuffix(email, "@users.noreply.github.com")
			_, after, found := strings.Cut(before, "+")
			if found {
				author.Username = after
			}
		}

		coAuthors = append(coAuthors, author)
	}

	return coAuthors, nil
}

func createAuthors(log *slog.Logger, githubClient github.Userer, event github.Event) (string, error) {
	// sender has login/username and url
	// commit co-authors only have username or name
	// author has name, email, and username
	compareAuthorFunc := func(author github.Author) func(github.Author) bool {
		return func(other github.Author) bool {
			return (author.Name != "" && strings.EqualFold(author.Name, other.Name)) || (author.Username != "" && strings.EqualFold(author.Username, other.Username))
		}
	}

	// First we gather all the authors of the commits
	commitAuthors := []github.Author{}
	for _, commit := range event.Commits {
		author := commit.Author
		if !slices.ContainsFunc(commitAuthors, compareAuthorFunc(author)) {
			commitAuthors = append(commitAuthors, author)
		}
	}

	// Then we gather all the co-authors of the commits, since authors have the
	// username we don't want add both of them at the same time in case an
	// co-author is also an author in a later commit
	commitCoAuthors := []github.Author{}
	for _, commit := range event.Commits {
		coAuthors, err := fetchCoAuthors(commit.Message)
		if err != nil {
			log.Error("Failed to fetch co-authors", "error", err)
		}

		for _, coAuthor := range coAuthors {
			if slices.ContainsFunc(commitAuthors, compareAuthorFunc(coAuthor)) {
				continue
			}

			commitCoAuthors = append(commitCoAuthors, coAuthor)
		}
	}

	// If there are just a handful of co-authors without username we can try to fetch them by their Nav e-mail
	if len(commitCoAuthors) < 6 {
		for i, coAuthor := range commitCoAuthors {
			if strings.HasSuffix(coAuthor.Email, "@nav.no") {
				userWithEmail, err := githubClient.GetUserByEmail(coAuthor.Email)
				if err != nil {
					log.Error("Failed to get user by email", "email", coAuthor.Email, "error", err)
				}

				if userWithEmail != nil {
					commitCoAuthors[i].Username = userWithEmail.Login
				}
			}
		}
	}

	for _, coAuthor := range commitCoAuthors {
		if !slices.ContainsFunc(commitAuthors, compareAuthorFunc(coAuthor)) {
			commitAuthors = append(commitAuthors, coAuthor)
		}
	}

	authorsAsString := make([]string, len(commitAuthors))
	for i, author := range commitAuthors {
		authorsAsString[i] = author.AsUser().ToSlack()
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
	authors, err := createAuthors(log, githubClient, event)
	if err != nil {
		return nil, fmt.Errorf("creating authors: %w", err)
	}

	text := fmt.Sprintf("<%s|%d new commits> pushed to `%s` by %s.", event.Compare, len(event.Commits), event.Repository.ToSlack(), authors)

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
	message.Attachments[0].FooterIcon = neutralGithubIcon
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
