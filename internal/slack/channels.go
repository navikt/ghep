package slack

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/navikt/ghep/internal/github"
)

func (c Client) EnsureChannels(teams map[string]github.Team) error {
	joinedChannels, err := c.listJoinedChannels()
	if err != nil {
		return err
	}

	for name, team := range teams {
		team.SlackChannels.Commits = c.findChannelIDByName(team.SlackChannels.Commits, joinedChannels)
		team.SlackChannels.Issues = c.findChannelIDByName(team.SlackChannels.Issues, joinedChannels)
		team.SlackChannels.PullRequests = c.findChannelIDByName(team.SlackChannels.PullRequests, joinedChannels)
		team.SlackChannels.Workflows = c.findChannelIDByName(team.SlackChannels.Workflows, joinedChannels)
		team.Config.ExternalContributorsChannel = c.findChannelIDByName(team.Config.ExternalContributorsChannel, joinedChannels)
		team.SlackChannels.Releases = c.findChannelIDByName(team.SlackChannels.Releases, joinedChannels)
		teams[name] = team
	}

	return nil
}

func (c Client) findChannelIDByName(channel string, joinedChannels map[string]string) string {
	if channel != "" {
		channel = strings.TrimPrefix(channel, "#")
		id, joined := joinedChannels[channel]
		if joined {
			return id
		}

		c.log.Warn("channel not joined", "channel", channel)
	}

	return channel
}

func (c Client) listJoinedChannels() (map[string]string, error) {
	response, err := c.listChannels("users.conversations")
	if err != nil {
		return nil, fmt.Errorf("listing all channels: %w", err)
	}

	channels := make(map[string]string, len(response))
	for index := range response {
		channels[response[index].Name] = response[index].ID
	}

	return channels, nil
}

func (c Client) listChannels(apiMethod string) ([]Channel, error) {
	req, err := http.NewRequest("GET", slackApi+"/"+apiMethod, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	query := req.URL.Query()
	query.Set("exclude_archived", "true")
	query.Set("limit", "200")
	query.Set("types", "public_channel,private_channel")

	var channels []Channel
	for {
		req.URL.RawQuery = query.Encode()

		resp, err := c.httpDoWithRetry(req, 3)
		if err != nil {
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var slackResp ChannelResponse
		if err := json.Unmarshal(body, &slackResp); err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusTooManyRequests {
				retryAfterHeader := resp.Header.Get("Retry-After")
				if retryAfterHeader != "" {
					retryAfter, err := time.ParseDuration(retryAfterHeader + "s")
					if err != nil {
						return nil, err
					}

					c.log.Info("rate limited, sleeping", "status", resp.Status, "error", slackResp.Error, "retry_after", retryAfter.Seconds())
					time.Sleep(retryAfter)
					continue
				}

				c.log.Info("rate limited and no Retry-After header set, sleeping 5 second")
				time.Sleep(5 * time.Second)
				continue
			}

			return nil, fmt.Errorf("non 200 status code(%v): %v", resp.StatusCode, slackResp.Error)
		}

		if !slackResp.Ok {
			return nil, fmt.Errorf("non OK: %v (needed=%s, provded=%s)", slackResp.Error, slackResp.Needed, slackResp.Provided)
		}

		if slackResp.Warning != "" {
			c.log.Info("got a warning", "warn", slackResp.Warning)
		}

		channels = append(channels, slackResp.Channels...)
		c.log.Info(fmt.Sprintf("Found %d channels", len(channels)), "new_channels", len(slackResp.Channels))

		if slackResp.ResponseMetadata.NextCursor == "" {
			break
		}

		query.Set("cursor", slackResp.ResponseMetadata.NextCursor)
		time.Sleep(3 * time.Second)
	}

	return channels, nil
}

func (c Client) JoinChannel(channel string) error {
	payload := map[string]string{
		"channel": channel,
	}

	marshalled, err := json.Marshal(payload)
	if err != nil {
		c.log.Error("Error marshalling payload", "error", err)
		return err
	}

	_, err = c.postRequest("conversations.join", marshalled)
	return err
}
