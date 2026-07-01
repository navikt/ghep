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

func RunPersonalDigestScheduler(ctx context.Context, log *slog.Logger, db *gensql.Queries, slackClient slack.Client, users []github.PersonalDigestUserEntry) {
	if len(users) == 0 {
		log.Info("No users configured for personal digest, scheduler not running")
		return
	}

	log.Info("Starting personal digest scheduler", "users", len(users))

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			for _, entry := range users {
				go func(e github.PersonalDigestUserEntry) {
					if err := maybeFirePersonalDigestForUser(ctx, log, db, slackClient, e, now); err != nil {
						log.Error("Sending personal digest", "login", e.Login, "error", err)
					}
				}(entry)
			}
		}
	}
}

func maybeFirePersonalDigestForUser(ctx context.Context, log *slog.Logger, db *gensql.Queries, slackClient slack.Client, entry github.PersonalDigestUserEntry, now time.Time) error {
	loc, err := time.LoadLocation(entry.Timezone)
	if err != nil {
		return err
	}

	local := now.In(loc)

	targetWeekday, ok := weekdays[strings.ToLower(entry.Day)]
	if !ok {
		return nil
	}

	if local.Weekday() != targetWeekday {
		return nil
	}

	// Compute the exact scheduled time for today in the user's timezone
	scheduledAt := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	parsed, err := time.ParseInLocation("15:04", entry.Time, loc)
	if err != nil {
		return err
	}
	scheduledAt = scheduledAt.Add(time.Duration(parsed.Hour())*time.Hour + time.Duration(parsed.Minute())*time.Minute)

	if now.Before(scheduledAt) {
		return nil
	}

	// Read the previous sent_at to determine the "since" window for commit lookup.
	prevSentAt, err := db.GetPersonalDigestSentAt(ctx, entry.Login)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	// Atomically claim this personal digest slot. If another goroutine already
	// claimed it (returned sent_at >= scheduledAt), bail out without sending.
	claimedAt, err := db.ClaimPersonalDigestSlot(ctx, gensql.ClaimPersonalDigestSlotParams{
		Login:       entry.Login,
		SentAt:      pgtype.Timestamptz{Time: now, Valid: true},
		ScheduledAt: pgtype.Timestamptz{Time: scheduledAt, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // already claimed this week
		}
		return err
	}
	if !claimedAt.Time.Equal(now) {
		return nil
	}

	return sendPersonalDigest(ctx, log, db, slackClient, entry.Login, now, prevSentAt)
}

func sendPersonalDigest(ctx context.Context, log *slog.Logger, db *gensql.Queries, slackClient slack.Client, login string, now time.Time, prevSentAt pgtype.Timestamptz) error {
	// Determine the time window: since last digest, or 7 days if never sent.
	var since pgtype.Timestamptz
	if prevSentAt.Valid {
		since = prevSentAt
	} else {
		since = pgtype.Timestamptz{Time: now.Add(-7 * 24 * time.Hour), Valid: true}
	}

	repos, err := db.GetUserCommitsSince(ctx, gensql.GetUserCommitsSinceParams{
		Login:        login,
		LastPushedAt: since,
	})
	if err != nil {
		return err
	}

	if len(repos) == 0 {
		return nil
	}

	slackID, err := db.GetUserSlackID(ctx, login)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Debug("No Slack ID for user, skipping personal digest", "login", login)
			return nil
		}
		return err
	}

	dmChannel, err := slackClient.OpenDM(slackID)
	if err != nil {
		return err
	}

	msg := slack.CreatePersonalDigestMessage(dmChannel, repos)
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if _, err := slackClient.PostMessage(payload); err != nil {
		return err
	}

	log.Info("Personal digest sent", "login", login, "repos", len(repos))

	return nil
}
