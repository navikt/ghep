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
	teams      []*github.Team
	orgMembers []*github.User

	ExternalContributorsChannel string
	SubscribeToOrg              bool
}

func New(log *slog.Logger, events events.Handler, redis *redis.Client, teams []*github.Team, orgMembers []*github.User, externalContributorsChannel string, subscribeToOrg bool) Client {
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
			c.orgMembers = append(c.orgMembers, &event.Membership.User)
		case "member_removed":
			c.orgMembers = slices.DeleteFunc(c.orgMembers, func(user *github.User) bool {
				return user.Login == event.Membership.User.Login
			})
		}

		fmt.Fprint(w, "Event handled for org")
		return
	}

	if (isAnExternalContributor(event.Sender, c.orgMembers) && c.ExternalContributorsChannel != "") || event.SecurityAdvisory != nil {
		team := &github.Team{
			Name: "external-contributors",
			SlackChannels: github.SlackChannels{
				PullRequests: c.ExternalContributorsChannel,
				Issues:       c.ExternalContributorsChannel,
			},
			Members: []*github.User{
				&event.Sender,
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

	var teams []*github.Team
	if c.SubscribeToOrg {
		teams = append(teams, c.teams[0])
	} else {
		if event.Team != nil {
			team, found := findTeamByName(c.teams, event.Team.Name)
			if !found {
				fmt.Fprintf(w, "No team found for event %s", event.Team.Name)
				return
			}

			teams = append(teams, team)
		} else {
			teams = findTeamsByRepository(c.teams, event.GetRepositoryName())
			if len(teams) == 0 {
				fmt.Fprintf(w, "No team found for repository %s", event.GetRepositoryName())
				return
			}
		}
	}

	for _, team := range teams {
		log = log.With("repository", event.GetRepositoryName(), "team", team.Name, "action", event.Action)
		if err := c.events.Handle(r.Context(), log, team, event); err != nil {
			log.Error("error handling event", "team", team.Name, "err", err.Error())
			http.Error(w, fmt.Sprintf("Error handling event for %s", team.Name), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "Event handled for team %s", team.Name)
	}
}

func findTeamByName(teams []*github.Team, teamName string) (*github.Team, bool) {
	for _, team := range teams {
		if team.Name == teamName {
			return team, true
		}
	}

	return nil, false
}

func findTeamsByRepository(teams []*github.Team, repositoryName string) []*github.Team {
	if repositoryName == "" {
		return []*github.Team{}
	}

	found := []*github.Team{}
	for _, team := range teams {
		if slices.Contains(team.Repositories, repositoryName) {
			found = append(found, team)
		}
	}

	return found
}

func isAnExternalContributor(user github.User, orgMembers []*github.User) bool {
	if user.IsBot() {
		return false
	}

	for _, member := range orgMembers {
		if member.Login == user.Login {
			return false
		}
	}

	return true
}
