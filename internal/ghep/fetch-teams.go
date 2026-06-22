package ghep

import (
	"context"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/sql/gensql"
)

func FetchGithubData(ctx context.Context, log *slog.Logger, db *gensql.Queries, teamConfig map[string]github.Team, githubClient github.Client, subscribeToOrg bool) {
	log.Info("Fetching data from Github")

	storedTeams, err := db.ListTeams(ctx)
	if err != nil {
		log.Error("Listing teams from database", "error", err)
		return
	}

	for name := range teamConfig {
		if slices.Contains(storedTeams, name) {
			continue
		}

		log.Info("Adding team to database", "team", name)
		if err := db.CreateTeam(ctx, name); err != nil {
			log.Error("Creating team in database", "team", name, "error", err)
			return
		}
	}

	for _, storedTeam := range storedTeams {
		if _, ok := teamConfig[storedTeam]; ok {
			continue
		}

		log.Info("Team no longer in config, deleting from database", "team", storedTeam)
		if err := db.DeleteTeam(ctx, storedTeam); err != nil {
			log.Error("Deleting team from database", "team", storedTeam, "error", err)
			return
		}
	}

	log.Info("Getting info about teams from Github")
	reposBlocklist := strings.Split(os.Getenv("GITHUB_BLOCKLIST_REPOS"), ",")

	if subscribeToOrg {
		if err := githubClient.FetchOrgAsTeam(ctx, log, reposBlocklist); err != nil {
			log.Error("Fetching org members from Github", "error", err)
			return
		}
	} else {
		if err := githubClient.FetchTeams(ctx, log, reposBlocklist); err != nil {
			log.Error("Fetching teams from Github", "error", err)
			return
		}
	}

	if err := githubClient.FetchOrgUsersWithEmail(ctx); err != nil {
		log.Error("Fetching org members from Github", "error", err)
		return
	}
}
