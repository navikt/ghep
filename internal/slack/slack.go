package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	slackApi = "https://slack.com/api"
)

type Attachment struct {
	Text       string `json:"text"`
	Color      string `json:"color"`
	Footer     string `json:"footer,omitempty"`
	FooterIcon string `json:"footer_icon,omitempty"`
}

type Message struct {
	Channel         string       `json:"channel"`
	Text            string       `json:"text"`
	Attachments     []Attachment `json:"attachments,omitempty"`
	ThreadTimestamp string       `json:"thread_ts,omitempty"`
	UnfurlLinks     bool         `json:"unfurl_links"`
	UnfurlMedia     bool         `json:"unfurl_media"`
}

type Channel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type responseData struct {
	Ok               bool      `json:"ok"`
	Error            string    `json:"error"`
	Nedded           string    `json:"needed"`
	Provided         string    `json:"provided"`
	Warn             string    `json:"warning"`
	Channels         []Channel `json:"channels"`
	TimeStamp        string    `json:"ts"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
	Message struct {
		Reactions []struct {
			Name string `json:"name"`
		} `json:"reactions"`
	} `json:"message"`
}

type Client struct {
	log        *slog.Logger
	httpClient *http.Client
	token      string
}

func New(log *slog.Logger, token string) (Client, error) {
	if token == "" {
		return Client{}, fmt.Errorf("missing Slack token")
	}

	client := Client{
		log: log,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		token: token,
	}

	return client, nil
}

func (c Client) postRequest(apiMethod string, payload []byte) (*responseData, error) {
	req, err := http.NewRequest("POST", slackApi+"/"+apiMethod, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	resp, err := c.httpDoWithRetry(req, 3)
	if err != nil {
		return nil, fmt.Errorf("giving up after 3 retries: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return c.handleSlackResponse(resp, string(body))
}

func (c Client) getRequest(apiMethod, channel, timestamp string) (*responseData, error) {
	req, err := http.NewRequest("GET", slackApi+"/"+apiMethod, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	query := req.URL.Query()
	query.Set("channel", channel)
	query.Set("timestamp", timestamp)
	req.URL.RawQuery = query.Encode()

	resp, err := c.httpDoWithRetry(req, 3)
	if err != nil {
		return nil, fmt.Errorf("giving up after 3 retries: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return c.handleSlackResponse(resp, string(body))

}

func (c Client) handleSlackResponse(resp *http.Response, body string) (*responseData, error) {
	var slackResp responseData
	if err := json.Unmarshal([]byte(body), &slackResp); err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return &slackResp, fmt.Errorf("non 200 status code(%v): %v", resp.StatusCode, slackResp.Error)
	}

	if !slackResp.Ok {
		return &slackResp, fmt.Errorf("non OK: %v (needed=%s, provded=%s)", slackResp.Error, slackResp.Nedded, slackResp.Provided)
	}

	if slackResp.Warn != "" {
		c.log.Info("got a warning", "warn", slackResp.Warn)
	}

	return &slackResp, nil
}

func (c Client) httpDoWithRetry(req *http.Request, retries int) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if retries == 0 {
			return nil, err
		}

		time.Sleep(1 * time.Second)

		return c.httpDoWithRetry(req, retries-1)
	}

	return resp, nil
}
