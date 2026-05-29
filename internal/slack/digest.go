package slack

import (
	"fmt"
	"strings"
	"time"

	"github.com/navikt/ghep/internal/github"
)

// CreateDigestMessage returns a summary message and an optional thread message.
// If there are no open PRs, only the summary message is returned (threadMsg is nil).
// Otherwise, the summary contains a short header with totals, and threadMsg contains
// the full repo/PR breakdown to be posted as a thread reply.
func CreateDigestMessage(channel string, repoPRs []github.RepoPRs) (summary *Message, threadMsg *Message) {
	if len(repoPRs) == 0 {
		return &Message{
			Channel: channel,
			Text:    "Gratulerer! Alle pull requests er merget – dere er helt à jour! :tada:",
		}, nil
	}

	now := time.Now()
	months := []string{
		"", "januar", "februar", "mars", "april", "mai", "juni",
		"juli", "august", "september", "oktober", "november", "desember",
	}
	dateStr := fmt.Sprintf("%d. %s %d", now.Day(), months[now.Month()], now.Year())

	totalPRs := 0
	for _, repo := range repoPRs {
		totalPRs += len(repo.PRs)
	}
	repoUnit := "repos"
	if len(repoPRs) == 1 {
		repoUnit = "repo"
	}
	prUnit := "åpne pull requests"
	if totalPRs == 1 {
		prUnit = "åpen pull request"
	}
	summaryText := fmt.Sprintf("*Ukentlig PR-oversikt — %s*\n%d %s med %d %s", dateStr, len(repoPRs), repoUnit, totalPRs, prUnit)

	var sb strings.Builder
	for _, repo := range repoPRs {
		count := len(repo.PRs)
		unit := "åpne"
		if count == 1 {
			unit = "åpen"
		}
		fmt.Fprintf(&sb, "*%s* (%d %s)\n", repo.RepoName, count, unit)
		for _, pr := range repo.PRs {
			fmt.Fprintf(&sb, "• <%s|#%d %s>\n", pr.URL, pr.Number, pr.Title)
		}
		sb.WriteString("\n")
	}

	return &Message{
			Channel: channel,
			Text:    summaryText,
		}, &Message{
			Channel: channel,
			Text:    strings.TrimRight(sb.String(), "\n"),
		}
}
