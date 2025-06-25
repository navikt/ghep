package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/navikt/ghep/internal/github"
)

const (
	slackApi          = "https://slack.com/api"
	neutralGithubIcon = "https://slack-imgs.com/?c=1&o1=wi32.he32.si&url=https%3A%2F%2Fslack.github.com%2Fstatic%2Fimg%2Ffavicon-neutral.png"

	ColorCritical = "#d1242f"
	ColorHigh     = "#bc4c00"
	ColorMedium   = "#9a6700"
	ColorLow      = "#34a44c"
	ColorDefault  = "#000000"
	ColorDraft    = "#eeeeee"
	ColorMerged   = "#7044c4"
	ColorOpened   = "#34a44c"
	ColorClosed   = "#d02434"
	ColorFailed   = "#d02434"
)

func getColorBySeverity(severity github.SeverityType) string {
	switch severity {
	case github.SeverityCritical:
		return ColorCritical
	case github.SeverityHigh:
		return ColorHigh
	case github.SeverityMedium:
		return ColorMedium
	case github.SeverityLow:
		return ColorLow
	default:
		return ColorDefault
	}
}

type Attachment struct {
	Text       string `json:"text"`
	Type       string `json:"type,omitempty"`
	Color      string `json:"color"`
	Footer     string `json:"footer,omitempty"`
	FooterIcon string `json:"footer_icon,omitempty"`
}

type Message struct {
	Channel         string       `json:"channel"`
	Text            string       `json:"text"`
	Attachments     []Attachment `json:"attachments,omitempty"`
	ThreadTimestamp string       `json:"thread_ts,omitempty"`
	Timestamp       string       `json:"ts,omitempty"`
	UnfurlLinks     bool         `json:"unfurl_links"`
	UnfurlMedia     bool         `json:"unfurl_media"`
}

type Channel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Response struct {
	Ok       bool   `json:"ok"`
	Error    string `json:"error"`
	Warning  string `json:"warning"`
	Needed   string `json:"needed"`
	Provided string `json:"provided"`

	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

type MessageResponse struct {
	Response

	Channel   string `json:"channel"`
	Timestamp string `json:"ts"`
}

type ReactionResponse struct {
	Response

	Message struct {
		Reactions []struct {
			Name string `json:"name"`
		} `json:"reactions"`
	} `json:"message"`
}

type ChannelResponse struct {
	Response

	Channels []Channel `json:"channels"`
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

func (c Client) postRequest(apiMethod string, payload []byte) (string, error) {
	req, err := http.NewRequest("POST", slackApi+"/"+apiMethod, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	resp, err := c.httpDoWithRetry(req, 3)
	if err != nil {
		return "", fmt.Errorf("giving up after 3 retries: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.log.Error("error reading response body", "error", err)
		return "", err
	}

	return c.handleSlackResponse(resp, string(body))
}

func (c Client) getRequest(apiMethod, channel, timestamp string) (string, error) {
	req, err := http.NewRequest("GET", slackApi+"/"+apiMethod, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	query := req.URL.Query()
	query.Set("channel", channel)
	query.Set("timestamp", timestamp)
	req.URL.RawQuery = query.Encode()

	resp, err := c.httpDoWithRetry(req, 3)
	if err != nil {
		return "", fmt.Errorf("giving up after 3 retries: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return c.handleSlackResponse(resp, string(body))
}

func (c Client) handleSlackResponse(resp *http.Response, body string) (string, error) {
	var slackResp Response
	if err := json.Unmarshal([]byte(body), &slackResp); err != nil {
		c.log.Error("error unmarshalling response", "error", err, "body", body)
		return body, err
	}

	if resp.StatusCode != 200 {
		return body, fmt.Errorf("non 200 status code(%v): %v", resp.StatusCode, slackResp.Error)
	}

	if !slackResp.Ok {
		return body, fmt.Errorf("non OK: %v (needed=%s, provded=%s)", slackResp.Error, slackResp.Needed, slackResp.Provided)
	}

	if slackResp.Warning != "" {
		c.log.Info("got a warning", "warn", slackResp.Warning)
	}

	return body, nil
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
