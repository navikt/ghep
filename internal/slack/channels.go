package slack

import (
	"fmt"
	"log/slog"

	"github.com/navikt/ghep/internal/github"
)

func (c Client) EnsureChannels(teams []github.Team) error {
	channels, err := c.GetChannels()
	if err != nil {
		return err
	}

	for _, team := range teams {
		if team.SlackChannels.Commits != "" {
			id, ok := channels[team.SlackChannels.Commits]
			if ok {
				c.JoinChannel(id)
			} else {
				slog.Warn("Commits channel not found", "channel", team.SlackChannels.Commits, "team", team.Name)
			}
		}
		if team.SlackChannels.Issues != "" {
			id, ok := channels[team.SlackChannels.Issues]
			if ok {
				c.JoinChannel(id)
			} else {
				slog.Warn("Issues channel not found", "channel", team.SlackChannels.Issues, "team", team.Name)
			}
		}
		if team.SlackChannels.PullRequests != "" {
			id, ok := channels[team.SlackChannels.PullRequests]
			if ok {
				c.JoinChannel(id)
			} else {
				slog.Warn("Pull requests channel not found", "channel", team.SlackChannels.PullRequests, "team", team.Name)
			}
		}
		if team.SlackChannels.Workflows != "" {
			id, ok := channels[team.SlackChannels.Workflows]
			if ok {
				c.JoinChannel(id)
			} else {
				slog.Warn("Workflows channel not found", "channel", team.SlackChannels.Workflows, "team", team.Name)
			}
		}
	}

	return nil
}

func (c Client) GetChannels() (map[string]string, error) {
	response, err := c.ListChannels()
	if err != nil {
		return nil, fmt.Errorf("listing channels: %w", err)
	}

	channels := make(map[string]string, len(response))
	for index := range response {
		channels[response[index].Name] = response[index].ID
	}

	return channels, nil
}
