package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const graphqlURL = "https://api.github.com/graphql"

type PullRequest struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
}

type RepoPRs struct {
	RepoName string
	PRs      []PullRequest
}

type graphqlPRNode struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	IsDraft   bool      `json:"isDraft"`
	CreatedAt time.Time `json:"createdAt"`
}

type graphqlPRConnection struct {
	Nodes    []graphqlPRNode `json:"nodes"`
	PageInfo struct {
		HasNextPage bool   `json:"hasNextPage"`
		EndCursor   string `json:"endCursor"`
	} `json:"pageInfo"`
}

type graphqlResponse struct {
	Data   map[string]json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (c Client) FetchOpenPullRequests(ctx context.Context, teamSlug string) ([]RepoPRs, error) {
	bearerToken, err := c.createBearerToken()
	if err != nil {
		return nil, fmt.Errorf("creating bearer token: %v", err)
	}

	repos, err := c.db.ListTeamRepositories(ctx, teamSlug)
	if err != nil {
		return nil, fmt.Errorf("listing repositories for team %s: %v", teamSlug, err)
	}

	if len(repos) == 0 {
		return nil, nil
	}

	httpClient := http.Client{Timeout: 30 * time.Second}

	repoNames := make([]string, len(repos))
	for i, r := range repos {
		repoNames[i] = r.Name
	}

	// Fetch first page for all repos in one query
	connections, err := fetchPRsBatch(ctx, httpClient, bearerToken, c.org, repoNames, nil)
	if err != nil {
		return nil, err
	}

	var result []RepoPRs
	for repoName, conn := range connections {
		var prs []PullRequest
		for _, n := range conn.Nodes {
			if !n.IsDraft {
				prs = append(prs, PullRequest{Number: n.Number, Title: n.Title, URL: n.URL, CreatedAt: n.CreatedAt})
			}
		}

		// Paginate repos that have more than 100 open PRs
		for conn.PageInfo.HasNextPage {
			cursor := conn.PageInfo.EndCursor
			more, err := fetchPRsBatch(ctx, httpClient, bearerToken, c.org, []string{repoName}, map[string]string{repoName: cursor})
			if err != nil {
				return nil, err
			}
			conn = more[repoName]
			for _, n := range conn.Nodes {
				if !n.IsDraft {
					prs = append(prs, PullRequest{Number: n.Number, Title: n.Title, URL: n.URL, CreatedAt: n.CreatedAt})
				}
			}
		}

		if len(prs) > 0 {
			result = append(result, RepoPRs{RepoName: repoName, PRs: prs})
		}
	}

	return result, nil
}

// fetchPRsBatch sends a single GraphQL query fetching open PRs for all given repos.
// cursors maps repoName -> after-cursor for pagination (nil or missing = first page).
func fetchPRsBatch(ctx context.Context, httpClient http.Client, bearerToken, org string, repoNames []string, cursors map[string]string) (map[string]graphqlPRConnection, error) {
	query := buildBatchQuery(org, repoNames, cursors)

	payload, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", graphqlURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+bearerToken)
	req.Header.Add("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close() // #nosec G104 -- closing response body, error intentionally ignored
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("graphql request failed (%v): %s", resp.Status, body)
	}

	var gqlResp graphqlResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("unmarshalling graphql response: %v", err)
	}
	if len(gqlResp.Errors) > 0 {
		msgs := make([]string, len(gqlResp.Errors))
		for i, e := range gqlResp.Errors {
			msgs[i] = e.Message
		}
		return nil, fmt.Errorf("graphql errors: %s", strings.Join(msgs, "; "))
	}

	result := make(map[string]graphqlPRConnection, len(repoNames))
	for i, repoName := range repoNames {
		alias := repoAlias(i)
		raw, ok := gqlResp.Data[alias]
		if !ok {
			continue
		}
		var repoData struct {
			PullRequests graphqlPRConnection `json:"pullRequests"`
		}
		if err := json.Unmarshal(raw, &repoData); err != nil {
			return nil, fmt.Errorf("unmarshalling repo %s: %v", repoName, err)
		}
		result[repoName] = repoData.PullRequests
	}

	return result, nil
}

func buildBatchQuery(org string, repoNames []string, cursors map[string]string) string {
	var sb strings.Builder
	sb.WriteString("query {")
	for i, name := range repoNames {
		after := ""
		if cursors != nil {
			if cursor, ok := cursors[name]; ok {
				after = fmt.Sprintf(`, after: "%s"`, cursor)
			}
		}
		fmt.Fprintf(&sb, `
  %s: repository(owner: %q, name: %q) {
    pullRequests(states: OPEN, first: 100%s) {
      nodes { number title url isDraft createdAt }
      pageInfo { hasNextPage endCursor }
    }
  }`, repoAlias(i), org, name, after)
	}
	sb.WriteString("\n}")
	return sb.String()
}

func repoAlias(i int) string {
	return fmt.Sprintf("repo_%d", i)
}
