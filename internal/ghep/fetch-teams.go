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
		log.Error("listing teams from database", "err", err.Error())
		return
	}

	for name := range teamConfig {
		if slices.Contains(storedTeams, name) {
			continue
		}

		log.Info("Adding team to database", "team", name)
		if err := db.CreateTeam(ctx, name); err != nil {
			log.Error("creating team in database", "team", name, "err", err.Error())
			return
		}
	}

	log.Info("Getting info about teams from Github")

	if subscribeToOrg {
		if err := githubClient.FetchOrgMembersAsTeam(ctx, log); err != nil {
			log.Error("fetching org members from Github", "err", err.Error())
			return
		}
	} else {
		if err := githubClient.FetchTeams(ctx, log, os.Getenv("GITHUB_BLOCKLIST_REPOS")); err != nil {
			log.Error("fetching teams from Github", "err", err.Error())
			return
		}
	}

	if err := githubClient.FetchOrgMembers(ctx, log); err != nil {
		log.Error("fetching org members from Github", "err", err.Error())
		return
	}
}
