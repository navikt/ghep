package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/navikt/ghep/internal/sql/gensql"
)

const (
	graphqlEndpoint = "https://api.github.com/graphql"
	allUserGraphQL  = `query FetchUsersWithEmail($org: String!, $cursor: String) {
	 organization(login: $org) {
	   samlIdentityProvider {
		 externalIdentities(first: 100, after: $cursor) {
		   nodes {
			 user {
			   login
               email
			 }
			 samlIdentity {
			   username
			 }
		   }
           pageInfo {
             endCursor
           }
		 }
	   }
	 }
	}`
)

type githubResponse struct {
	Data struct {
		Organization struct {
			SamlIdentityProvider struct {
				ExternalIdentities struct {
					Nodes []struct {
						User struct {
							Login string `json:"login"`
							Email string `json:"email"`
						} `json:"user"`
						SamlIdentity struct {
							Email string `json:"username"`
						} `json:"samlIdentity"`
					} `json:"nodes"`
					PageInfo struct {
						EndCursor string `json:"endCursor"`
					} `json:"pageInfo"`
				} `json:"externalIdentities"`
			} `json:"samlIdentityProvider"`
		} `json:"organization"`
	} `json:"data"`
	Errors []struct {
		Type    string   `json:"type"`
		Path    []string `json:"path"`
		Message string   `json:"message"`
	} `json:"errors"`
}

func (c *Client) FetchOrgUsersWithEmail(ctx context.Context) error {
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
