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

func (c Client) EnsureChannels(teams []github.Team) error {
	channels, err := c.GetAllChannels()
	if err != nil {
		return err
	}

	joinedChannels, err := c.GetJoinedChannels()
	if err != nil {
		return err
	}

	for i, team := range teams {
		teams[i].SlackChannels.Commits = c.ensureChannel(team.SlackChannels.Commits, channels, joinedChannels)
		teams[i].SlackChannels.Issues = c.ensureChannel(team.SlackChannels.Issues, channels, joinedChannels)
		teams[i].SlackChannels.PullRequests = c.ensureChannel(team.SlackChannels.PullRequests, channels, joinedChannels)
		teams[i].SlackChannels.Workflows = c.ensureChannel(team.SlackChannels.Workflows, channels, joinedChannels)
		teams[i].Config.ExternalContributorsChannel = c.ensureChannel(team.Config.ExternalContributorsChannel, channels, joinedChannels)
		teams[i].SlackChannels.Releases = c.ensureChannel(team.SlackChannels.Releases, channels, joinedChannels)
	}

	return nil
}

func (c Client) ensureChannel(channel string, channels map[string]string, joinedChannels map[string]string) string {
	if channel != "" {
		channel = strings.TrimPrefix(channel, "#")
		id, joined := joinedChannels[channel]
		if joined {
			return id
		}

		id, ok := channels[channel]
		if ok {
			if err := c.JoinChannel(id); err != nil {
				c.log.Error("ensuring channels", "channel", channel, "error", err)
			}
			return id
		}

		c.log.Warn("channel not found", "channel", channel)
	}

	return channel
}

func (c Client) GetJoinedChannels() (map[string]string, error) {
	return c.getAndParseChannels("users.conversations")
}

func (c Client) GetAllChannels() (map[string]string, error) {
	// TODO: Dette blir heftig rate limited ved oppstart. Kan jeg løse dette på en bedre måte?
	// return c.getAndParseChannels("conversations.list")
	return map[string]string{}, nil
}

func (c Client) getAndParseChannels(apiMethod string) (map[string]string, error) {
	response, err := c.ListChannels(apiMethod)
	if err != nil {
		return nil, fmt.Errorf("listing all channels: %w", err)
	}

	channels := make(map[string]string, len(response))
	for index := range response {
		channels[response[index].Name] = response[index].ID
	}

	return channels, nil
}

func (c Client) ListChannels(apiMethod string) ([]Channel, error) {
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

		var slackResp responseData
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

					c.log.Info("rate limited, sleeping", "status", resp.Status, "error", slackResp.Error, "retry_after", retryAfter)
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

		if slackResp.Warn != "" {
			c.log.Info("got a warning", "warn", slackResp.Warn)
		}

		channels = append(channels, slackResp.Channels...)
		c.log.Info(fmt.Sprintf("Found %d channels", len(channels)))

		if slackResp.ResponseMetadata.NextCursor == "" {
			break
		}

		query.Set("cursor", slackResp.ResponseMetadata.NextCursor)
	}

	return channels, nil
}

func (c Client) JoinChannel(channel string) error {
	payload := map[string]string{
		"channel": channel,
	}

	marshalled, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_, err = c.postRequest("conversations.join", marshalled)
	return err
}
