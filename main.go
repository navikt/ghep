package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
)

type client struct {
	slack slack.Client
}

func main() {
	slackApi, err := slack.New(os.Getenv("SLACK_TOKEN"))
	if err != nil {
		slog.Error("error creating Slack client", "err", err.Error())
		os.Exit(1)
	}

	api := client{
		slack: slackApi,
	}

	slog.Info("Starting server")
	http.HandleFunc("/events", api.events)
	if err := http.ListenAndServe("127.0.0.1:8080", nil); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

type SimpleEvent struct {
	Repository struct {
		Name string `json:"name"`
	} `json:"repository"`
	Zen string `json:"zen"`
}

func (c client) events(w http.ResponseWriter, r *http.Request) {
	slog.Info("Received event")
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("error reading body", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	simpleEvent := SimpleEvent{}
	if err := json.Unmarshal(body, &simpleEvent); err != nil {
		slog.Error("error decoding body", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if simpleEvent.Zen != "" {
		slog.Info("Received ping event")
		w.WriteHeader(http.StatusOK)
		return
	}

	if simpleEvent.Repository.Name != "" {
		name := simpleEvent.Repository.Name
		slog.Info(fmt.Sprintf("Received repository event for repository: %v", name))
		// TODO: Add check for repository name
		// if name == "crm-nks-integration" {
		if err != nil {
			slog.Error("error reading body", "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		commit, err := github.CreateCommitEvent(body)
		if err != nil {
			slog.Error("error creating commit event", "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if len(commit.Commits) == 0 {
			slog.Info("No commits to post")
			w.WriteHeader(http.StatusOK)
			return
		}

		payload, err := slack.CreateCommitMessage("#nada-test", commit)
		if err != nil {
			slog.Error("error creating Slack message", "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := c.slack.PostMessage(payload); err != nil {
			slog.Error("error posting to Slack", "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		slog.Info("Successfully posted to Slack")
		// }
	}

	w.WriteHeader(http.StatusOK)
}
