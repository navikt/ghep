package slack

import (
	"fmt"
	"strings"
	"time"

	"github.com/navikt/ghep/internal/github"
)

func CreateDigestMessage(channel string, repoPRs []github.RepoPRs) (summary *Message, threadMsgs []*Message) {
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

	for _, repo := range repoPRs {
		var sb strings.Builder
		count := len(repo.PRs)
		unit := "åpne"
		if count == 1 {
			unit = "åpen"
		}
		fmt.Fprintf(&sb, "*%s* (%d %s)\n", repo.RepoName, count, unit)
		for _, pr := range repo.PRs {
			days := int(time.Since(pr.CreatedAt).Hours() / 24)
			dayUnit := "dager"
			if days == 1 {
				dayUnit = "dag"
			}
			fmt.Fprintf(&sb, "• <%s|#%d %s> (%d %s)\n", pr.URL, pr.Number, pr.Title, days, dayUnit)
		}
		threadMsgs = append(threadMsgs, &Message{
			Channel: channel,
			Text:    strings.TrimRight(sb.String(), "\n"),
		})
	}

	return &Message{
		Channel: channel,
		Text:    summaryText,
	}, threadMsgs
}
