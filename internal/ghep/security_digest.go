package ghep

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/navikt/ghep/internal/sql/gensql"
)

func RunSecurityDigestScheduler(ctx context.Context, log *slog.Logger, db *gensql.Queries, teamConfig map[string]github.Team, githubClient github.Client, slackClient slack.Client) {
	type securityDigestEntry struct {
		teamSlug string
		digest   *github.SecurityDigestConfig
	}

	var entries []securityDigestEntry
	for slug, team := range teamConfig {
		if team.SecurityDigest != nil {
			entries = append(entries, securityDigestEntry{teamSlug: slug, digest: team.SecurityDigest})
		}
	}

	if len(entries) == 0 {
		log.Info("No teams configured for security digest, scheduler not running")
		return
	}

	log.Info("Starting security digest scheduler", "teams", len(entries))

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			for _, entry := range entries {
				go func(e securityDigestEntry) {
					if err := maybeFireSecurityDigest(ctx, log, db, now, e.teamSlug, e.digest, teamConfig, githubClient, slackClient); err != nil {
						log.Error("Sending security digest", "team", e.teamSlug, "error", err)
					}
				}(entry)
			}
		}
	}
}

func maybeFireSecurityDigest(ctx context.Context, log *slog.Logger, db *gensql.Queries, now time.Time, teamSlug string, digest *github.SecurityDigestConfig, teamConfig map[string]github.Team, githubClient github.Client, slackClient slack.Client) error {
	tz := digest.Timezone
	if tz == "" {
		tz = "Europe/Oslo"
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		return err
	}

	local := now.In(loc)

	targetWeekday, ok := weekdays[strings.ToLower(digest.Day)]
	if !ok {
		return nil
	}

	if local.Weekday() != targetWeekday {
		return nil
	}

	// Compute the exact scheduled time for today in the team's timezone.
	scheduledAt := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	parsed, err := time.ParseInLocation("15:04", digest.Time, loc)
	if err != nil {
		return err
	}
	scheduledAt = scheduledAt.Add(time.Duration(parsed.Hour())*time.Hour + time.Duration(parsed.Minute())*time.Minute)

	// Too early — scheduled time hasn't arrived yet.
	if now.Before(scheduledAt) {
		return nil
	}

	// Check DB: skip if already sent after the most recent scheduled time.
	sentAt, err := db.GetSecurityDigestSentAt(ctx, teamSlug)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if sentAt.Valid && sentAt.Time.After(scheduledAt) {
		return nil
	}

	log.Info("Sending security digest", "team", teamSlug, "channel", digest.Channel)

	team, exists := teamConfig[teamSlug]
	var globalIgnore []string
	if exists {
		globalIgnore = team.Config.IgnoreRepositories
	}

	repoAlerts, err := githubClient.FetchOpenSecurityAlerts(ctx, teamSlug, digest, globalIgnore)
	if err != nil {
		return err
	}

	totalAlerts := 0
	for _, r := range repoAlerts {
		totalAlerts += r.Total()
	}

	if totalAlerts == 0 && !digest.SendEmpty {
		log.Info("No security alerts to digest", "team", teamSlug, "channel", digest.Channel)
	} else {
		teamName := ""
		if digest.SpecifyTeamName {
			teamName = github.TitleCaseSlug(teamSlug)
		}
		summary, threadMsgs := slack.CreateSecurityDigestMessage(digest.Channel, teamName, repoAlerts)

		payload, err := json.Marshal(summary)
		if err != nil {
			return err
		}

		resp, err := slackClient.PostMessage(payload)
		if err != nil {
			return err
		}

		for _, threadMsg := range threadMsgs {
			threadMsg.ThreadTimestamp = resp.Timestamp
			threadPayload, err := json.Marshal(threadMsg)
			if err != nil {
				return err
			}
			if _, err := slackClient.PostMessage(threadPayload); err != nil {
				return err
			}
		}
	}

	if err := db.UpsertSecurityDigestSent(ctx, gensql.UpsertSecurityDigestSentParams{
		TeamSlug: teamSlug,
		SentAt:   pgtype.Timestamptz{Time: now, Valid: true},
	}); err != nil {
		// Log but don't fail — message was already sent successfully.
		log.Error("Upserting security digest sent timestamp", "team", teamSlug, "error", err)
	}

	log.Info("Security digest sent", "team", teamSlug, "channel", digest.Channel, "total_alerts", totalAlerts)

	return nil
}
