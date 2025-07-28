package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/navikt/ghep/internal/events"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/sql/gensql"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	log        *slog.Logger
	db         *gensql.Queries
	events     events.Handler
	redis      *redis.Client
	teamConfig map[string]github.Team

	ExternalContributorsChannel string
	SubscribeToOrg              bool
}

func New(log *slog.Logger, db *gensql.Queries, events events.Handler, redis *redis.Client, teamConfig map[string]github.Team, externalContributorsChannel string, subscribeToOrg bool) Client {
	return Client{
		log:        log,
		db:         db,
		events:     events,
		redis:      redis,
		teamConfig: teamConfig,

		ExternalContributorsChannel: externalContributorsChannel,
		SubscribeToOrg:              subscribeToOrg,
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

	if slices.Contains([]string{"member_added", "member_removed"}, event.Action) {
		log.Info("Handling org event", "action", event.Action, "user", event.Membership.User.Login, "triggered_by", event.Sender.Login)
		switch event.Action {
		case "member_added":
			if err := c.db.CreateUser(r.Context(), event.Membership.User.Login); err != nil {
				log.Error("error creating user in database", "user", event.Membership.User.Login, "err", err.Error())
				http.Error(w, fmt.Sprintf("Error creating user in database: %s", err.Error()), http.StatusInternalServerError)
				return
			}
		case "member_removed":
			if err := c.db.DeleteUser(r.Context(), event.Membership.User.Login); err != nil {
				log.Error("error deleting user from database", "user", event.Membership.User.Login, "err", err.Error())
				http.Error(w, fmt.Sprintf("Error deleting user from database: %s", err.Error()), http.StatusInternalServerError)
				return
			}
		}

		fmt.Fprint(w, "Event handled for org")
		return
	}

	isAnExternalContributor, err := c.isAnExternalContributor(r.Context(), event.Sender)
	if err != nil {
		log.Error("error checking if user is an external contributor", "user", event.Sender.Login, "err", err.Error())
		http.Error(w, fmt.Sprintf("Error checking if user is an external contributor: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	if (isAnExternalContributor && c.ExternalContributorsChannel != "") || event.SecurityAdvisory != nil {
		team := github.Team{
			Name: "external-contributors",
			SlackChannels: github.SlackChannels{
				PullRequests: c.ExternalContributorsChannel,
				Issues:       c.ExternalContributorsChannel,
			},
		}

		log := log.With("repository", event.GetRepositoryName(), "team", team.Name, "action", event.Action, "user", event.Sender.Login, "external_contributor", true)
		log.Info("Handling event for external contributors")
		if err := c.events.Handle(r.Context(), log, team, event); err != nil {
			log.Error("error handling event for external contributors", "err", err.Error())
			http.Error(w, "Error handling event for external contributors", http.StatusInternalServerError)
			return
		}

		if event.SecurityAdvisory != nil {
			fmt.Fprintf(w, "Security advisory event handled")
			return
		}

	}

	var teams []string
	if c.SubscribeToOrg {
		for name := range c.teamConfig {
			teams = append(teams, name)
			break
		}
	} else {
		if event.Team != nil {
			team, err := c.db.GetTeam(r.Context(), event.Team.Name)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					log.Warn("team not found in database", "team", event.Team.Name)
				} else {
					log.Error("error getting team from database", "team", event.Team.Name, "err", err.Error())
				}

				fmt.Fprintf(w, "No team found for event %s", event.Team.Name)
				return
			}

			teams = append(teams, team)
		} else {
			teamsFromDB, err := c.db.ListTeamsByRepository(r.Context(), event.GetRepositoryName())
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					log.Warn("no teams found for repository in database", "repository", event.GetRepositoryName())
				} else {
					log.Error("error listing teams by repository", "repository", event.GetRepositoryName(), "err", err.Error())
					http.Error(w, fmt.Sprintf("Error listing teams by repository: %s", err.Error()), http.StatusInternalServerError)
					return
				}

				fmt.Fprintf(w, "No team found for repository %s", event.GetRepositoryName())
				return
			}

			teams = teamsFromDB
		}
	}

	for _, name := range teams {
		log = log.With("repository", event.GetRepositoryName(), "team", name, "action", event.Action)
		if err := c.events.Handle(r.Context(), log, c.teamConfig[name], event); err != nil {
			log.Error("error handling event", "team", name, "err", err.Error())
			http.Error(w, fmt.Sprintf("Error handling event for %s", name), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "Event handled for team %s", name)
	}
}

func (c *Client) isAnExternalContributor(ctx context.Context, user github.User) (bool, error) {
	if user.IsBot() {
		return false, nil
	}

	_, err := c.db.GetUser(ctx, user.Login)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
