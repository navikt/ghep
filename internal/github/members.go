package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/navikt/ghep/internal/sql/gensql"
)

// fetchMembers fetches members for a Github org or team using the following API:
// Team: https://docs.github.com/en/rest/teams/members#list-team-members
// Org: https://docs.github.com/en/rest/orgs/members#list-organization-members
func fetchMembers(teamURL, bearerToken string) ([]*User, error) {
	req, err := http.NewRequest("GET", teamURL+"/members", nil)
	if err != nil {
		return nil, err
	}

	query := req.URL.Query()
	query.Set("per_page", "100")

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", bearerToken))
	req.Header.Add("Content-Type", "application/json")

	httpClient := http.Client{
		Timeout: 10 * time.Second,
	}

	var teamMembers []*User
	page := 1
	for {
		query.Set("page", strconv.Itoa(page))
		req.URL.RawQuery = query.Encode()

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s returned %v: %s", teamURL, resp.Status, body)
		}

		var members []*User
		if err := json.Unmarshal(body, &members); err != nil {
			return nil, err
		}

		teamMembers = append(teamMembers, members...)

		if len(members) < 100 {
			break
		}

		page++
	}

	return teamMembers, nil
}

func (c *Client) FetchOrgMembersWithEmail(ctx context.Context) error {
	bearerToken, err := c.createBearerToken()
	if err != nil {
		return fmt.Errorf("creating bearer token: %v", err)
	}

	query := map[string]any{
		"query": allUserGraphQL,
		"variables": map[string]string{
			"org":    c.org,
			"cursor": "",
		},
	}

	httpClient := http.Client{
		Timeout: 10 * time.Second,
	}

	for {
		body, err := json.Marshal(query)
		if err != nil {
			return fmt.Errorf("marshalling query: %v", err)
		}

		req, err := http.NewRequest(http.MethodPost, graphqlEndpoint, bytes.NewBuffer(body))
		if err != nil {
			return err
		}

		req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", bearerToken))
		req.Header.Add("Content-Type", "application/json")

		httpResp, err := httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("doing request: %v", err)
		}
		defer httpResp.Body.Close()

		var githubResp githubResponse
		if err := json.NewDecoder(httpResp.Body).Decode(&githubResp); err != nil {
			return fmt.Errorf("decoding response: %v", err)
		}

		if httpResp.StatusCode != http.StatusOK {
			return fmt.Errorf("error fetching user (%v): %s", httpResp.Status, githubResp.Errors)
		}

		if len(githubResp.Errors) > 0 {
			var b strings.Builder
			for _, err := range githubResp.Errors {
				fmt.Fprintf(&b, "%s (type=%s, path=[%s])\n", err.Message, err.Type, strings.Join(err.Path, " "))
			}

			return fmt.Errorf("graphql error: %s", b.String())
		}

		identities := githubResp.Data.Organization.SamlIdentityProvider.ExternalIdentities
		for _, identity := range identities.Nodes {
			user := identity.User
			if err := c.db.CreateUser(ctx, user.Login); err != nil {
				return fmt.Errorf("creating user %s: %w", user.Login, err)
			}

			if user.Email != "" {
				if err := c.db.CreateEmail(ctx, gensql.CreateEmailParams{
					Login: user.Login,
					Email: user.Email,
				}); err != nil {
					return fmt.Errorf("creating email for user %s: %w", user.Login, err)
				}
			}

			if err := c.db.CreateEmail(ctx, gensql.CreateEmailParams{
				Login: user.Login,
				Email: identity.SamlIdentity.Email,
			}); err != nil {
				return fmt.Errorf("creating email for user %s: %w", user.Login, err)
			}
		}

		if identities.PageInfo.EndCursor != "" {
			cursor := identities.PageInfo.EndCursor
			query["variables"].(map[string]string)["cursor"] = cursor
		} else {
			return nil
		}
	}
}
