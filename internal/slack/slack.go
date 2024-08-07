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
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

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

	templates, err := initSlackTemplates()
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

func initSlackTemplates() (map[string]template.Template, error) {
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

func (c Client) PostMessage(payload []byte) (string, error) {
	req, err := http.NewRequest("POST", "https://slack.com/api/chat.postMessage", bytes.NewReader(payload))
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

	type slackResponse struct {
		Ok        bool   `json:"ok"`
		Error     string `json:"error"`
		Warn      string `json:"warning"`
		TimeStamp string `json:"ts"`
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("error posting message to Slack(%v): %v", resp.Status, body)
	}

	var slackResp slackResponse
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
