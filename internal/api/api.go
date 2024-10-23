package api

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/navikt/ghep/internal/events"
	"github.com/navikt/ghep/internal/github"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	events events.Handler
	redis  *redis.Client
	teams  []github.Team
}

func New(events events.Handler, redis *redis.Client, teams []github.Team) Client {
	return Client{
		events: events,
		redis:  redis,
		teams:  teams,
	}
}

func (c Client) Run(addr string) error {
	slog.Info("Starting server")
	http.HandleFunc("POST /events", c.eventsPostHandler)
	http.HandleFunc("GET /internal/health", c.healthGetHandler)
	return http.ListenAndServe(addr, nil)
}

func (c *Client) eventsPostHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("error reading body", "err", err.Error())
		fmt.Fprintf(w, "error reading body: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	event, err := github.CreateEvent(body)
	if err != nil {
		slog.Error("error creating event", "err", err.Error())
		fmt.Fprintf(w, "error creating event: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var team github.Team
	if event.Team == nil {
		var found bool

		team, found = findTeam(c.teams, event.Repository.Name)
		if !found {
			fmt.Fprintf(w, "No team found for repository %s", event.Repository.Name)
			return
		}
	}

	log := slog.With("repository", event.Repository.Name, "team", team.Name, "action", event.Action)
	if err := c.events.Handle(r.Context(), log, team, event); err != nil {
		log.Error("error handling event", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "%s event handled for team %s", event.Repository.Name, team.Name)
}

func findTeam(teams []github.Team, repositoryName string) (github.Team, bool) {
	for _, team := range teams {
		for _, repo := range team.Repositories {
			if repo == repositoryName {
				return team, true
			}
		}
	}

	return github.Team{}, false
}
