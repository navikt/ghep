package ghep

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/navikt/ghep/internal/sql/gensql"
)

func isLeader() (bool, error) {
	electorURL := os.Getenv("ELECTOR_GET_URL")
	if electorURL == "" {
		return true, nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(electorURL)
	if err != nil {
		return false, fmt.Errorf("querying elector: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("decoding elector response: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return false, fmt.Errorf("getting hostname: %w", err)
	}

	return result.Name == hostname, nil
}

func RunLeaderSchedulers(
	ctx context.Context,
	log *slog.Logger,
	db *gensql.Queries,
	teamConfig map[string]github.Team,
	githubClient github.Client,
	slackClient slack.Client,
	personalDigestUsers []github.PersonalDigestUserEntry,
) {
	var cancelSchedulers context.CancelFunc

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	// Check immediately on startup rather than waiting a full minute.
	checkAndUpdate := func() {
		leader, err := isLeader()
		if err != nil {
			log.Error("Checking leader election", "error", err)
			return
		}

		if leader && cancelSchedulers == nil {
			log.Info("Elected as leader, starting digest schedulers")
			schedulerCtx, cancel := context.WithCancel(ctx)
			cancelSchedulers = cancel

			go RunPersonalDigestScheduler(schedulerCtx, log.With("component", "personal-digest"), db, slackClient, personalDigestUsers)
			go RunPullRequestDigestScheduler(schedulerCtx, log.With("component", "pr-digest"), db, teamConfig, githubClient, slackClient)
			go RunSecurityDigestScheduler(schedulerCtx, log.With("component", "security-digest"), db, teamConfig, githubClient, slackClient)
		} else if !leader && cancelSchedulers != nil {
			log.Info("Lost leadership, stopping digest schedulers")
			cancelSchedulers()
			cancelSchedulers = nil
		}
	}

	checkAndUpdate()

	for {
		select {
		case <-ctx.Done():
			if cancelSchedulers != nil {
				cancelSchedulers()
			}
			return
		case <-ticker.C:
			checkAndUpdate()
		}
	}
}
