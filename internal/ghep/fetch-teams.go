package ghep

import (
	"context"
	"log/slog"
	"os"
	"slices"

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

	log.Info("Getting info about teams from Github")

	if subscribeToOrg {
		if err := githubClient.FetchOrgAsTeam(ctx, log); err != nil {
			log.Error("Fetching org members from Github", "error", err)
			return
		}
	} else {
		if err := githubClient.FetchTeams(ctx, log, os.Getenv("GITHUB_BLOCKLIST_REPOS")); err != nil {
			log.Error("Fetching teams from Github", "error", err)
			return
		}
	}

	if err := githubClient.FetchOrgUsersWithEmail(ctx); err != nil {
		log.Error("Fetching org members from Github", "error", err)
		return
	}
}
