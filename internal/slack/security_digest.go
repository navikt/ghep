package slack

import (
	"fmt"
	"strings"
	"time"

	"github.com/navikt/ghep/internal/github"
)

func CreateSecurityDigestMessage(channel string, repoAlerts []github.RepoSecurityAlerts) (summary *Message, threadMsgs []*Message) {
	if len(repoAlerts) == 0 {
		return &Message{
			Channel: channel,
			Text:    "Gratulerer! Ingen åpne sikkerhetsvarsler – dere er helt sikre! :tada:",
		}, nil
	}

	now := time.Now()
	months := []string{
		"", "januar", "februar", "mars", "april", "mai", "juni",
		"juli", "august", "september", "oktober", "november", "desember",
	}
	dateStr := fmt.Sprintf("%d. %s %d", now.Day(), months[now.Month()], now.Year())

	var totalSecret, totalCode, totalDependabot int
	for _, r := range repoAlerts {
		totalSecret += len(r.SecretScanning)
		totalCode += len(r.CodeScanning)
		totalDependabot += len(r.Dependabot)
	}
	total := totalSecret + totalCode + totalDependabot

	repoUnit := "repos"
	if len(repoAlerts) == 1 {
		repoUnit = "repo"
	}
	alertUnit := "åpne sikkerhetsvarsler"
	if total == 1 {
		alertUnit = "åpent sikkerhetsvarsel"
	}

	var breakdown strings.Builder
	parts := []string{}
	if totalSecret > 0 {
		parts = append(parts, fmt.Sprintf("%d secret scanning", totalSecret))
	}
	if totalCode > 0 {
		parts = append(parts, fmt.Sprintf("%d code scanning", totalCode))
	}
	if totalDependabot > 0 {
		parts = append(parts, fmt.Sprintf("%d Dependabot", totalDependabot))
	}
	if len(parts) > 0 {
		breakdown.WriteString(" (")
		breakdown.WriteString(strings.Join(parts, ", "))
		breakdown.WriteString(")")
	}

	summaryText := fmt.Sprintf("*Ukentlig sikkerhetsdigest — %s*\n%d %s på tvers av %d %s%s",
		dateStr, total, alertUnit, len(repoAlerts), repoUnit, breakdown.String())

	summary = &Message{
		Channel: channel,
		Text:    summaryText,
	}

	var totalCriticals int

	// One thread message per repo
	for _, repo := range repoAlerts {
		var sb strings.Builder
		fmt.Fprintf(&sb, "*%s*\n", repo.ToSlack())

		if len(repo.SecretScanning) > 0 {
			fmt.Fprintf(&sb, ":key: %s\n", repo.ToSlackWithMetadata("secret-scanning", len(repo.SecretScanning)))
		}

		var criticals int
		for _, a := range repo.CodeScanning {
			if github.AsSeverityType(a.Severity) == github.SeverityCritical {
				criticals += 1
			}
		}
		totalCriticals += criticals

		fmt.Fprintf(&sb, ":mag: %s", repo.ToSlackWithMetadata("code-scanning", len(repo.CodeScanning)))
		addCriticalText(&sb, criticals)

		criticals = 0
		for _, a := range repo.Dependabot {
			if github.AsSeverityType(a.Severity) == github.SeverityCritical {
				criticals += 1
			}
		}
		totalCriticals += criticals

		fmt.Fprintf(&sb, ":dependabot: %s", repo.ToSlackWithMetadata("dependabot", len(repo.Dependabot)))
		addCriticalText(&sb, criticals)

		threadMsgs = append(threadMsgs, &Message{
			Channel: channel,
			Text:    sb.String(),
		})
	}

	if totalCriticals > 0 {
		criticalUnit := "criticals"
		if totalCriticals == 1 {
			criticalUnit = "critical"
		}

		summary.Text = summary.Text + fmt.Sprintf("\n:warning: %d %s :warning:", totalCriticals, criticalUnit)
	}

	return summary, threadMsgs
}

func addCriticalText(sb *strings.Builder, criticals int) {
	if criticals > 0 {
		if criticals == 1 {
			fmt.Fprintln(sb, " :warning: one critical :warning:")
		} else {
			fmt.Fprintf(sb, " :warning: %d criticals :warning:\n", criticals)
		}
	} else {
		fmt.Fprintln(sb, "")
	}
}
