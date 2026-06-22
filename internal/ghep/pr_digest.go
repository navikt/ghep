package ghep

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"time"

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

func RunPullRequestDigestScheduler(ctx context.Context, log *slog.Logger, db *gensql.Queries, teamConfig map[string]github.Team, githubClient github.Client, slackClient slack.Client) {
	type digestEntry struct {
		teamSlug string
		digest   *github.DigestConfig
	}

	var entries []digestEntry
	for slug, team := range teamConfig {
		if team.PullRequestDigest != nil {
			entries = append(entries, digestEntry{teamSlug: slug, digest: team.PullRequestDigest})
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
					if err := maybeFireDigest(ctx, log, db, now, e.teamSlug, e.digest, teamConfig, githubClient, slackClient); err != nil {
						log.Error("Sending digest", "team", e.teamSlug, "error", err)
					}
				}(entry)
			}
		}
	}
}

func maybeFireDigest(ctx context.Context, log *slog.Logger, db *gensql.Queries, now time.Time, teamSlug string, digest *github.DigestConfig, teamConfig map[string]github.Team, githubClient github.Client, slackClient slack.Client) error {
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

	// Atomically claim this digest slot. If another goroutine already claimed
	// it (returned sent_at >= scheduledAt), bail out without sending.
	claimedAt, err := db.ClaimDigestSlot(ctx, gensql.ClaimDigestSlotParams{
		TeamSlug:    teamSlug,
		SentAt:      pgtype.Timestamptz{Time: now, Valid: true},
		ScheduledAt: pgtype.Timestamptz{Time: scheduledAt, Valid: true},
	})
	if err != nil {
		return err
	}
	if !claimedAt.Time.Equal(now) {
		return nil
	}

	log.Info("Sending weekly digest", "team", teamSlug, "channel", digest.Channel)

	repoPRs, err := githubClient.FetchOpenPullRequests(ctx, teamSlug)
	if err != nil {
		return err
	}

	// Filter out ignored repositories (global config)
	team, exists := teamConfig[teamSlug]
	if exists && len(team.Config.IgnoreRepositories) > 0 {
		var filtered []github.RepoPRs
		for _, repoPR := range repoPRs {
			if !slices.Contains(team.Config.IgnoreRepositories, repoPR.RepoName) {
				filtered = append(filtered, repoPR)
			}
		}
		repoPRs = filtered
	}

	// Filter out digest-specific ignored repositories
	if len(digest.IgnoreRepositories) > 0 {
		var filtered []github.RepoPRs
		for _, repoPR := range repoPRs {
			if !slices.Contains(digest.IgnoreRepositories, repoPR.RepoName) {
				filtered = append(filtered, repoPR)
			}
		}
		repoPRs = filtered
	}

	if len(repoPRs) == 0 && !digest.SendEmpty {
		log.Info("No pull request to digest", "team", teamSlug, "channel", digest.Channel)
	} else {
		teamName := ""
		if digest.SpecifyTeamName {
			teamName = github.TitleCaseSlug(teamSlug)
		}
		summary, threadMsgs := slack.CreatePullRequestDigestMessage(digest.Channel, teamName, repoPRs)

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

	log.Info("Digest sent", "team", teamSlug, "channel", digest.Channel, "repos_with_prs", len(repoPRs))

	return nil
}
