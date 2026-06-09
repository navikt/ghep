package slack

import (
	"fmt"
	"strings"
	"time"

	"github.com/navikt/ghep/internal/sql/gensql"
)

func CreatePersonalDigestMessage(channelID string, repos []gensql.GetUserCommitsSinceRow) *Message {
	now := time.Now()
	months := []string{
		"", "januar", "februar", "mars", "april", "mai", "juni",
		"juli", "august", "september", "oktober", "november", "desember",
	}
	dateStr := fmt.Sprintf("%d. %s %d", now.Day(), months[now.Month()], now.Year())

	var totalCommits int32
	for _, r := range repos {
		totalCommits += r.CommitCount
	}

	repoUnit := "repos"
	if len(repos) == 1 {
		repoUnit = "repo"
	}

	commitUnit := "commits"
	if totalCommits == 1 {
		commitUnit = "commit"
	}

	header := fmt.Sprintf("*Din ukentlige commit-oversikt — %s*\n%d %s med %d %s totalt",
		dateStr, len(repos), repoUnit, totalCommits, commitUnit)

	var sb strings.Builder
	for _, r := range repos {
		c := r.CommitCount
		cu := "commits"
		if c == 1 {
			cu = "commit"
		}
		fmt.Fprintf(&sb, "• *%s* — %d %s\n", r.Repo, c, cu)
	}

	return &Message{
		Channel: channelID,
		Text:    header + "\n\n" + strings.TrimRight(sb.String(), "\n"),
	}
}
