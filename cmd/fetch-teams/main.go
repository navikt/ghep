package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/sql"
)

func main() {
	ctx := context.Background()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	log.Info(fmt.Sprintf("Fetching data from Github for %s", os.Getenv("GITHUB_ORG")))

	db, err := sql.New(ctx, log.With("component", "db"), false)
	if err != nil {
		log.Error("creating SQL client", "err", err.Error())
		os.Exit(1)
	}

	log.Info("Parsing team configuration")
	teams, err := github.ParseTeamConfig(os.Getenv("REPOS_CONFIG_FILE_PATH"))
	if err != nil {
		log.Error("parsing team config", "err", err.Error())
		os.Exit(1)
	}

	storedTeams, err := db.ListTeams(ctx)
	if err != nil {
		log.Error("listing teams from database", "err", err.Error())
		os.Exit(1)
	}

	for name := range teams {
		if slices.Contains(storedTeams, name) {
			continue
		}

		log.Info("Adding team to database", "team", name)
		if err := db.CreateTeam(ctx, name); err != nil {
			log.Error("creating team in database", "team", name, "err", err.Error())
			os.Exit(1)
		}
	}

	log.Info("Getting info about teams from Github")
	githubClient := github.New(
		log.With("component", "github"),
		db,
		os.Getenv("GITHUB_APP_INSTALLATION_ID"),
		os.Getenv("GITHUB_APP_ID"),
		os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		os.Getenv("GITHUB_ORG"),
	)

	subscribeToOrg, _ := strconv.ParseBool(os.Getenv("GHEP_SUBSCRIBE_TO_ORG"))
	if err := githubClient.FetchTeams(
		ctx,
		os.Getenv("GITHUB_BLOCKLIST_REPOS"),
		os.Getenv("GITHUB_ORG"),
		subscribeToOrg,
	); err != nil {
		log.Error("fetching teams from Github", "err", err.Error())
		os.Exit(1)
	}
}
