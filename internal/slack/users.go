package slack

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func (c Client) ListUsers() ([]User, error) {
	req, err := http.NewRequest("GET", slackApi+"/users.list", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	query := req.URL.Query()
	query.Set("exclude_archived", "true")
	query.Set("limit", "1000")

	var users []User
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

		var slackResp UserResponse
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

		for _, user := range slackResp.Users {
			users = append(users, User{
				ID:    user.ID,
				Email: user.Profile.Email,
			})
		}
		c.log.Info(fmt.Sprintf("Found %d users", len(users)), "new_users_added", len(slackResp.Users))

		if slackResp.ResponseMetadata.NextCursor == "" {
			break
		}

		query.Set("cursor", slackResp.ResponseMetadata.NextCursor)
		time.Sleep(2 * time.Second)
	}

	c.log.Info(fmt.Sprintf("Total users fetched: %d", len(users)))
	return users, nil
}
