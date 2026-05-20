package slack

import (
	"fmt"
	"strings"
	"time"

	"github.com/navikt/ghep/internal/github"
)

func CreateDigestMessage(channel string, repoPRs []github.RepoPRs) *Message {
	var text string

	if len(repoPRs) == 0 {
		text = "Gratulerer! Alle pull requests er merget – dere er helt à jour! :tada:"
	} else {
		now := time.Now()
		months := []string{
			"", "januar", "februar", "mars", "april", "mai", "juni",
			"juli", "august", "september", "oktober", "november", "desember",
		}
		dateStr := fmt.Sprintf("%d. %s %d", now.Day(), months[now.Month()], now.Year())

		var sb strings.Builder
		fmt.Fprintf(&sb, "*Ukentlig PR-oversikt — %s*\n", dateStr)

		for _, repo := range repoPRs {
			count := len(repo.PRs)
			unit := "åpne"
			if count == 1 {
				unit = "åpen"
			}
			fmt.Fprintf(&sb, "\n*%s* (%d %s)\n", repo.RepoName, count, unit)
			for _, pr := range repo.PRs {
				fmt.Fprintf(&sb, "• <%s|#%d %s>\n", pr.URL, pr.Number, pr.Title)
			}
		}

		text = strings.TrimRight(sb.String(), "\n")
	}

	return &Message{
		Channel: channel,
		Text:    text,
	}
}
