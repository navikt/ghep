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
	log    *slog.Logger
	events events.Handler
	redis  *redis.Client
	teams  []github.Team
}

func New(log *slog.Logger, events events.Handler, redis *redis.Client, teams []github.Team) Client {
	return Client{
		log:    log,
		events: events,
		redis:  redis,
		teams:  teams,
	}
}

func (c *Client) Run(base, addr string) error {
	http.HandleFunc(fmt.Sprintf("POST %s/events", base), c.eventsPostHandler)
	http.HandleFunc("GET /internal/health", c.healthGetHandler)
	return http.ListenAndServe(addr, nil)
}

func (c *Client) eventsPostHandler(w http.ResponseWriter, r *http.Request) {
	deliveryID := r.Header.Get("X-GitHub-Delivery")
	eventType := r.Header.Get("X-GitHub-Event")
	log := c.log.With("delivery_id", deliveryID, "event_type", eventType)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("error reading body", "err", err.Error())
		http.Error(w, fmt.Sprintf("error reading body: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	event, err := github.CreateEvent(body)
	if err != nil {
		log.Error("error creating event", "err", err.Error())
		http.Error(w, fmt.Sprintf("error creating event: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	var team github.Team
	if event.Team != nil {
		team.Name = event.Team.Name
	} else {
		var found bool

		team, found = findTeam(c.teams, event)
		if !found {
			fmt.Fprintf(w, "No team found for repository %s", event.Repository.Name)
			return
		}
	}

	log = log.With("repository", event.Repository.Name, "team", team.Name, "action", event.Action)
	if err := c.events.Handle(r.Context(), log, team, event); err != nil {
		log.Error("error handling event", "err", err.Error())
		http.Error(w, fmt.Sprintf("error handling event: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Event handled for team %s", team.Name)
}

func findTeam(teams []github.Team, event github.Event) (github.Team, bool) {
	repositoryName := event.FindRepositoryName()

	for _, team := range teams {
		for _, repo := range team.Repositories {
			if repo == repositoryName {
				return team, true
			}
		}
	}

	return github.Team{}, false
}
