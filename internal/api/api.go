package api

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"slices"

	"github.com/navikt/ghep/internal/events"
	"github.com/navikt/ghep/internal/github"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	log        *slog.Logger
	events     events.Handler
	redis      *redis.Client
	teams      []github.Team
	orgMembers []github.User

	ExternalContributorsChannel string
	SubscribeToOrg              bool
}

func New(log *slog.Logger, events events.Handler, redis *redis.Client, teams []github.Team, orgMembers []github.User, externalContributorsChannel string, subscribeToOrg bool) Client {
	return Client{
		log:        log,
		events:     events,
		redis:      redis,
		teams:      teams,
		orgMembers: orgMembers,

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
			c.orgMembers = append(c.orgMembers, event.Membership.User)
		case "member_removed":
			c.orgMembers = slices.DeleteFunc(c.orgMembers, func(user github.User) bool {
				return user.Login == event.Membership.User.Login
			})
		}

		fmt.Fprint(w, "Event handled for org")
		return
	}

	var team github.Team
	if isAnExternalContributor(event.Sender, c.orgMembers) && c.ExternalContributorsChannel != "" {
		team = github.Team{
			Name: "external-contributors",
			SlackChannels: github.SlackChannels{
				PullRequests: c.ExternalContributorsChannel,
				Issues:       c.ExternalContributorsChannel,
			},
			Members: []github.User{
				event.Sender,
			},
		}

		log := log.With("repository", event.Repository.Name, "team", team.Name, "action", event.Action)
		if err := c.events.Handle(r.Context(), log, team, event); err != nil {
			log.Error("error handling event for external contributors", "err", err.Error())
			http.Error(w, "Error handling event for external contributors", http.StatusInternalServerError)
		}
	}

	if c.SubscribeToOrg {
		team = c.teams[0]
	} else {
		if event.Team != nil {
			for _, t := range c.teams {
				if t.Name == event.Team.Name {
					team = t
					break
				}
			}
		} else {
			var found bool

			team, found = findTeamByRepository(c.teams, event.GetRepositoryName())
			if !found {
				fmt.Fprintf(w, "No team found for repository %s", event.GetRepositoryName())
				return
			}
		}
	}

	log = log.With("repository", event.GetRepositoryName(), "team", team.Name, "action", event.Action)
	if err := c.events.Handle(r.Context(), log, team, event); err != nil {
		log.Error("error handling event", "err", err.Error())
		http.Error(w, fmt.Sprintf("Error handling event for %s", team.Name), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Event handled for team %s", team.Name)
}

func findTeamByRepository(teams []github.Team, repositoryName string) (github.Team, bool) {
	if repositoryName == "" {
		return github.Team{}, false
	}

	for _, team := range teams {
		if slices.Contains(team.Repositories, repositoryName) {
			return team, true
		}
	}

	return github.Team{}, false
}

func isAnExternalContributor(user github.User, orgMembers []github.User) bool {
	if user.IsBot() {
		return false
	}

	return slices.ContainsFunc(orgMembers, func(member github.User) bool {
		return member.Login == user.Login
	})
}
