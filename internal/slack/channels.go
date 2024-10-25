package slack

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
				slog.Error("ensuring channels", "channel", channel, "error", err)
			}
			return id
		} else {
			slog.Warn("channel not found", "channel", channel)
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
	req, err := http.NewRequest("POST", slackApi+"/conversations.list", nil)
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

					slog.Info("rate limited when listing channels", "status", resp.Status, "error", slackResp.Error, "retry_after", retryAfter)
					time.Sleep(retryAfter)
					continue
				}

				slog.Info("rate limited when listing channel, no Retry-After header set, sleeping 5 second")
				time.Sleep(5 * time.Second)
				continue
			}

			return nil, fmt.Errorf("error listing channels(%v): %v", resp.Status, slackResp)
		}

		if !slackResp.Ok {
			return nil, fmt.Errorf("listing channels returned not ok: %v", slackResp.Error)
		}

		if slackResp.Warn != "" {
			slog.Info("warning listing channels", "warn", slackResp.Warn)
		}

		if slackResp.Channels == nil && len(slackResp.Channels) == 0 {
			return nil, fmt.Errorf("no channels found")
		}

		channels = append(channels, slackResp.Channels...)
		slog.Info(fmt.Sprintf("Found %d channels", len(channels)))

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

	_, err = c.request("conversations.join", marshalled)
	return fmt.Errorf("joining channel: %w", err)
}
