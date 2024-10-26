package slack

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/navikt/ghep/internal/github"
)

func (c Client) EnsureChannels(teams []github.Team) error {
	channels, err := c.GetChannels()
	if err != nil {
		return err
	}

	for i, team := range teams {
		teams[i].SlackChannels.Commits = c.ensureChannel(team.SlackChannels.Commits, channels)
		teams[i].SlackChannels.Issues = c.ensureChannel(team.SlackChannels.Issues, channels)
		teams[i].SlackChannels.PullRequests = c.ensureChannel(team.SlackChannels.PullRequests, channels)
		teams[i].SlackChannels.Workflows = c.ensureChannel(team.SlackChannels.Workflows, channels)
	}

	return nil
}

func (c Client) ensureChannel(channel string, channels map[string]string) string {
	if channel != "" {
		id, ok := channels[channel]
		if ok {
			if err := c.JoinChannel(id); err != nil {
				c.log.Error("ensuring channels", "channel", channel, "error", err)
			}
			return id
		} else {
			c.log.Warn("channel not found", "channel", channel)
		}
	}

	return channel
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

func (c Client) ListChannels() ([]Channel, error) {
	req, err := http.NewRequest("GET", slackApi+"/conversations.list", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	query := req.URL.Query()
	query.Set("exclude_archived", "true")
	query.Set("limit", "200")

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
		if err := json.Unmarshal([]byte(body), &slackResp); err != nil {
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
			return nil, fmt.Errorf("non OK: %v (needed=%s, provded=%s)", slackResp.Error, slackResp.Nedded, slackResp.Provided)
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
