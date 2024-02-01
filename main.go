package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/navikt/ghep/internal/api"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
)

func main() {
	teams, err := github.FetchTeams(
		os.Getenv("GITHUB_API"),
		os.Getenv("GITHUB_APP_INSTALLATION_ID"),
		os.Getenv("GITHUB_APP_ID"),
		os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		os.Getenv("GITHUB_ORG"),
		os.Getenv("REPOS_CONFIG_FILE_PATH"),
		os.Getenv("GITHUB_BLOCKLIST_REPOS"),
	)
	if err != nil {
		slog.Error("error fetching teams", "err", err.Error())
		os.Exit(1)
	}

	teamNames := []string{}
	for _, team := range teams {
		teamNames = append(teamNames, team.Name)
	}

	slog.Info(fmt.Sprintf("Teams using Ghep: %v", teamNames))

	slackApi, err := slack.New(os.Getenv("SLACK_TOKEN"))
	if err != nil {
		slog.Error("error creating Slack client", "err", err.Error())
		os.Exit(1)
	}

	api := api.New(teams, slackApi)

	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = "0.0.0.0:8080"
	}

	if err := api.Run(addr); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
