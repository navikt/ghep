package main

import (
	"context"
	"log/slog"
	"os"
	"strconv"

	"github.com/navikt/ghep/internal/ghep"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/navikt/ghep/internal/sql"
)

func main() {
	ctx := context.Background()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	db, err := sql.New(ctx, log.With("client", "db"), true)
	if err != nil {
		log.Error("Creating SQL client", "error", err)
		os.Exit(1)
	}

	log.Info("Parsing team configuration")
	teamConfig, personalDigestUsers, err := github.ParseTeamConfig(os.Getenv("REPOS_CONFIG_FILE_PATH"))
	if err != nil {
		log.Error("Parsing team config", "error", err)
		os.Exit(1)
	}

	githubClient := github.New(
		db,
		os.Getenv("GITHUB_APP_INSTALLATION_ID"),
		os.Getenv("GITHUB_APP_ID"),
		os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		os.Getenv("GITHUB_ORG"),
	)

	log.Info("Creating Slack client")
	slackClient, err := slack.New(
		log.With("client", "slack"),
		os.Getenv("SLACK_TOKEN"),
	)
	if err != nil {
		log.Error("Creating Slack client", "error", err)
		os.Exit(1)
	}

	subscribeToOrg, _ := strconv.ParseBool(os.Getenv("GHEP_SUBSCRIBE_TO_ORG"))

	go ghep.FetchGithubData(ctx, log.With("component", "fetch-teams"), db, teamConfig, githubClient, subscribeToOrg)
	go ghep.FetchSlackUsers(ctx, log.With("component", "fetch-slack"), db)
	go ghep.RunDigestScheduler(ctx, log.With("component", "digest"), db, teamConfig, githubClient, slackClient)
	go ghep.RunPersonalDigestScheduler(ctx, log.With("component", "personal-digest"), db, slackClient, personalDigestUsers)
	go ghep.RunSecurityDigestScheduler(ctx, log.With("component", "security-digest"), db, teamConfig, githubClient, slackClient)

	glog := log.With("component", "ghep")
	if err := ghep.Run(ctx, glog, db, teamConfig, githubClient, slackClient, subscribeToOrg); err != nil {
		glog.Error("Running Ghep", "error", err)
		os.Exit(1)
	}
}
