package main

import (
	"context"
	"log/slog"
	"os"
	"strconv"

	"github.com/navikt/ghep/internal/ghep"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/sql"
)

func main() {
	ctx := context.Background()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	db, err := sql.New(ctx, log.With("client", "db"), true)
	if err != nil {
		log.Error("creating SQL client", "err", err.Error())
		os.Exit(1)
	}

	log.Info("Parsing team configuration")
	teamConfig, err := github.ParseTeamConfig(os.Getenv("REPOS_CONFIG_FILE_PATH"))
	if err != nil {
		log.Error("parsing team config", "err", err.Error())
		os.Exit(1)
	}

	githubClient := github.New(
		db,
		os.Getenv("GITHUB_APP_INSTALLATION_ID"),
		os.Getenv("GITHUB_APP_ID"),
		os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		os.Getenv("GITHUB_ORG"),
	)

	subscribeToOrg, _ := strconv.ParseBool(os.Getenv("GHEP_SUBSCRIBE_TO_ORG"))

	go ghep.FetchGithubData(ctx, log.With("component", "fetch-teams"), db, teamConfig, githubClient, subscribeToOrg)

	glog := log.With("component", "ghep")
	if err := ghep.Run(ctx, glog, db, teamConfig, githubClient, subscribeToOrg); err != nil {
		glog.Error("running Ghep", "err", err.Error())
		os.Exit(1)
	}
}
