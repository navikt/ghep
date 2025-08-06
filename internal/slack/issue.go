package slack

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/sql"
)

func CreateIssueMessage(ctx context.Context, log *slog.Logger, db sql.Userer, channel, threadTimestamp string, pingSlack bool, event github.Event) *Message {
	color := ColorOpened

	text := fmt.Sprintf("Issue <%s|#%d> %s in `%s` by %s", event.Issue.URL, event.Issue.Number, event.Action, event.Repository.ToSlack(), event.Sender.ToSlack())
	attachmentText := fmt.Sprintf("*<%s|#%d %s>*", event.Issue.URL, event.Issue.Number, html.EscapeString(event.Issue.Title))

	if event.Action == "closed" {
		color = ColorMerged
		text = fmt.Sprintf("Issue <%s|#%d> %s as %s in `%s` by %s", event.Issue.URL, event.Issue.Number, event.Action, event.Issue.StateReason, event.Repository.ToSlack(), event.Sender.ToSlack())
	}

	if event.Action != "closed" && event.Issue.Body != "" {
		attachmentText = fmt.Sprintf("%s\n%s", attachmentText, event.Issue.Body)
	}

	if len(event.Issue.Assignees) > 0 {
		var assignees strings.Builder
		for i, assignee := range event.Issue.Assignees {
			if pingSlack {
				userID, err := db.GetUserSlackID(ctx, assignee.Login)
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					log.Error("error getting user Slack ID", "user", assignee.Login, "err", err.Error())
				}

				if userID != "" {
					fmt.Fprintf(&assignees, "<@%s>", userID)
				} else {
					fmt.Fprintf(&assignees, "@%s", assignee.Login)
				}
			} else {
				fmt.Fprintf(&assignees, "@%s", assignee.Login)
			}

			if i < len(event.Issue.Assignees)-1 {
				assignees.WriteString(", ")
			}
		}

		attachmentText += fmt.Sprintf("\n*Assignees:* %s", assignees.String())
	}

	return &Message{
		Channel:         channel,
		ThreadTimestamp: threadTimestamp,
		Text:            text,
		Attachments: []Attachment{
			{
				Text:       attachmentText,
				Type:       "mrkdwn",
				Color:      color,
				FooterIcon: neutralGithubIcon,
				Footer:     fmt.Sprintf("<%s|%s>", event.Repository.URL, event.Repository.FullName),
			},
		},
	}
}
