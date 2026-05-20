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

var weekdays = map[string]time.Weekday{
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
	"sunday":    time.Sunday,
}

func RunDigestScheduler(ctx context.Context, log *slog.Logger, db *gensql.Queries, teamConfig map[string]github.Team, githubClient github.Client, slackClient slack.Client) {
	type digestEntry struct {
		teamSlug string
		digest   *github.DigestConfig
	}

	var entries []digestEntry
	for slug, team := range teamConfig {
		if team.Digest != nil {
			entries = append(entries, digestEntry{teamSlug: slug, digest: team.Digest})
		}
	}

	if len(entries) == 0 {
		log.Info("No teams configured for digest, scheduler not running")
		return
	}

	log.Info("Starting digest scheduler", "teams", len(entries))

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			for _, entry := range entries {
				go func(e digestEntry) {
					if err := maybeFireDigest(ctx, log, db, now, e.teamSlug, e.digest, githubClient, slackClient); err != nil {
						log.Error("Sending digest", "team", e.teamSlug, "error", err)
					}
				}(entry)
			}
		}
	}
}

func maybeFireDigest(
	ctx context.Context,
	log *slog.Logger,
	db *gensql.Queries,
	now time.Time,
	teamSlug string,
	digest *github.DigestConfig,
	githubClient github.Client,
	slackClient slack.Client,
) error {
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

	// Compute the exact scheduled time for today in the team's timezone
	scheduledAt := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	parsed, err := time.ParseInLocation("15:04", digest.Time, loc)
	if err != nil {
		return err
	}
	scheduledAt = scheduledAt.Add(time.Duration(parsed.Hour())*time.Hour + time.Duration(parsed.Minute())*time.Minute)

	// Too early — scheduled time hasn't arrived yet
	if now.Before(scheduledAt) {
		return nil
	}

	// Check DB: skip if already sent after the most recent scheduled time
	sentAt, err := db.GetDigestSentAt(ctx, teamSlug)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if sentAt.Valid && sentAt.Time.After(scheduledAt) {
		return nil
	}

	log.Info("Sending weekly digest", "team", teamSlug, "channel", digest.Channel)

	repoPRs, err := githubClient.FetchOpenPullRequests(ctx, teamSlug)
	if err != nil {
		return err
	}

	if len(repoPRs) == 0 && !digest.SendEmpty {
		return nil
	}

	msg := slack.CreateDigestMessage(digest.Channel, repoPRs)

	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if _, err := slackClient.PostMessage(payload); err != nil {
		return err
	}

	if err := db.UpsertDigestSent(ctx, gensql.UpsertDigestSentParams{
		TeamSlug: teamSlug,
		SentAt:   pgtype.Timestamptz{Time: now, Valid: true},
	}); err != nil {
		// Log but don't fail — message was already sent successfully
		log.Error("Upserting digest sent timestamp", "team", teamSlug, "error", err)
	}

	log.Info("Digest sent", "team", teamSlug, "channel", digest.Channel, "repos_with_prs", len(repoPRs))

	return nil
}
