package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/sql"
)

func createAuthors(ctx context.Context, log *slog.Logger, db sql.Userer, event github.Event) (string, error) {
	// event sender has login/username and url
	// commit co-authors only have a name and e-mail
	// commit author has name, e-mail, and login/username
	compareAuthorFunc := func(author github.Author) func(github.Author) bool {
		return func(other github.Author) bool {
			return (author.Name != "" && strings.EqualFold(author.Name, other.Name)) ||
				(author.Username != "" && strings.EqualFold(author.Username, other.Username)) ||
				(author.Email != "" && strings.EqualFold(author.Email, other.Email))
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
	// username, we don't want add both of them at the same time, in case an
	// co-author is also an author in a later commit
	commitCoAuthors := []github.Author{}
	for _, commit := range event.Commits {
		coAuthors := github.FetchCoAuthors(commit.Message)

		for _, coAuthor := range coAuthors {
			if slices.ContainsFunc(commitAuthors, compareAuthorFunc(coAuthor)) {
				continue
			}

			commitCoAuthors = append(commitCoAuthors, coAuthor)
		}
	}

	for i, coAuthor := range commitCoAuthors {
		if strings.HasSuffix(coAuthor.Email, "@nav.no") {
			username, err := db.GetUserByEmail(ctx, coAuthor.Email)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				log.Error("Getting user by email", "email", coAuthor.Email, "error", err)
			}

			if username != "" {
				commitCoAuthors[i].Username = username
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

func CreateCommitMessage(ctx context.Context, log *slog.Logger, db sql.Userer, channel string, event github.Event) (*Message, error) {
	authors, err := createAuthors(ctx, log, db, event)
	if err != nil {
		return nil, fmt.Errorf("creating authors: %w", err)
	}

	text := fmt.Sprintf("<%s|%d new commits> pushed to `%s` by %s", event.Compare, len(event.Commits), event.Repository.ToSlack(), authors)

	var attachmentText strings.Builder
	for _, c := range event.Commits {
		firstLine := strings.Split(c.Message, "\n")[0]

		attachmentText.WriteString(fmt.Sprintf("`<%s|%s>` - %s\n", c.URL, c.ID[:8], firstLine))
	}

	attachments := []Attachment{
		{
			Text:  attachmentText.String(),
			Color: ColorDefault,
		},
	}

	return &Message{
		Channel:     channel,
		Text:        text,
		Attachments: attachments,
	}, nil
}

// CreateUpdatedCommitMessage sets the footer of the commit message to the workflow URL and name if it is not already set.
func CreateUpdatedCommitMessage(payload []byte, event github.Event) (*Message, error) {
	var message Message
	if err := json.Unmarshal(payload, &message); err != nil {
		return nil, fmt.Errorf("unmarshalling message: %w", err)
	}

	if len(message.Attachments) == 0 {
		return nil, nil
	}

	if message.Attachments[0].Footer != "" {
		return nil, nil
	}

	message.Attachments[0].FooterIcon = neutralGithubIcon
	message.Attachments[0].Footer = fmt.Sprintf("<%s|%s>", event.Workflow.URL, event.Workflow.Name)

	return &message, nil
}
