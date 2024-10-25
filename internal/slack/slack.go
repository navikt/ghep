package slack

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/navikt/ghep/internal/github"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

const (
	slackApi = "https://slack.com/api"
)

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
}

type Client struct {
	httpClient *http.Client
	token      string
	templates  map[string]template.Template
}

func (c Client) CommitTmpl() template.Template {
	return c.templates["commit"]
}

func (c Client) IssueTmpl() template.Template {
	return c.templates["issue"]
}

func (c Client) PullRequestTmpl() template.Template {
	return c.templates["pull"]
}

func (c Client) RemovedTmpl() template.Template {
	return c.templates["removed"]
}

func (c Client) RenamedTmpl() template.Template {
	return c.templates["renamed"]
}

func (c Client) TeamTmpl() template.Template {
	return c.templates["team"]
}

func (c Client) WorkflowTmpl() template.Template {
	return c.templates["workflow"]
}

func New(token string) (Client, error) {
	if token == "" {
		return Client{}, fmt.Errorf("missing Slack token")
	}

	templates, err := ParseMessageTemplates()
	if err != nil {
		return Client{}, err
	}

	client := Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		token:     token,
		templates: templates,
	}

	return client, nil
}

func ParseMessageTemplates() (map[string]template.Template, error) {
	templates := map[string]template.Template{}

	files, err := templatesFS.ReadDir("templates")
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		tmpl, err := template.ParseFS(templatesFS, "templates/"+file.Name())
		if err != nil {
			return nil, err
		}

		name := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
		templates[name] = *tmpl
	}

	return templates, nil
}

func (c Client) PostWorkflowReaction(log *slog.Logger, team github.Team, event github.Event, timestamp string) error {
	reaction := "dogcited"
	if event.Action == "requested" && event.Workflow.Status == "queued" {
		reaction = "eyes"
	}

	if event.Action == "in_progress" && event.Workflow.Status == "in_progress" {
		reaction = "hourglass_with_flowing_sand"
	}

	if event.Action == "completed" && event.Workflow.Conclusion == "success" {
		reaction = "white_check_mark"
	}

	if event.Action == "completed" && event.Workflow.Conclusion == "failure" {
		reaction = "x"
	}

	log.Info("Posting reaction to workflow event", "reaction", reaction, "channel", team.SlackChannels.Commits)
	return c.PostReaction(reaction, team.SlackChannels.Commits, timestamp)
}

func (c Client) PostReaction(channel, reaction, timestamp string) error {
	payload := map[string]string{
		"channel":   channel,
		"name":      reaction,
		"timestamp": timestamp,
	}

	marshalled, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", slackApi+"/reactions.add", bytes.NewReader(marshalled))
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	resp, err := c.httpDoWithRetry(req, 3)
	if err != nil {
		return err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("error reacting(%v)", resp.StatusCode)
	}

	var slackResp responseData
	if err := json.Unmarshal([]byte(body), &slackResp); err != nil {
		return fmt.Errorf("error unmarshal Slack response: %v, body: %v", err, body)
	}

	if !slackResp.Ok {
		return fmt.Errorf("error posting message to Slack: %v (needed=%s, provded=%s)", slackResp.Error, slackResp.Nedded, slackResp.Provided)
	}

	return nil
}

func (c Client) PostMessage(payload []byte) (string, error) {
	req, err := http.NewRequest("POST", slackApi+"/chat.postMessage", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	resp, err := c.httpDoWithRetry(req, 3)
	if err != nil {
		return "", err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("error posting message to Slack(%v): %v", resp.Status, body)
	}

	var slackResp responseData
	if err := json.Unmarshal([]byte(body), &slackResp); err != nil {
		return "", err
	}

	if !slackResp.Ok {
		return "", fmt.Errorf("error posting message to Slack: %v", slackResp.Error)
	}

	if slackResp.Warn != "" {
		slog.Info("warning posting message to Slack", "warn", slackResp.Warn)
	}

	return slackResp.TimeStamp, nil
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
				slog.Info("rate limited when listing channels", "status", resp.Status, "error", slackResp.Error, "headers", resp.Header)
				retryAfterHeader := resp.Header.Get("Retry-After")
				if retryAfterHeader != "" {
					retryAfter, err := time.ParseDuration(retryAfterHeader + "s")
					if err != nil {
						return nil, err
					}

					time.Sleep(retryAfter)
					continue
				}

				slog.Info("no Retry-After header, sleeping 5 second")
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

	req, err := http.NewRequest("POST", slackApi+"/conversations.join", bytes.NewReader(marshalled))
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	resp, err := c.httpDoWithRetry(req, 3)
	if err != nil {
		return err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("error joining channel(%v): %v", resp.Status, body)
	}

	var slackResp responseData
	if err := json.Unmarshal([]byte(body), &slackResp); err != nil {
		return err
	}

	if !slackResp.Ok {
		return fmt.Errorf("joining channel returned not ok: %v", slackResp.Error)
	}

	if slackResp.Warn != "" {
		slog.Info("warning joining channel", "warn", slackResp.Warn, "channel", channel)
	}

	return nil
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
