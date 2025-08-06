package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	graphqlEndpoint = "https://api.github.com/graphql"
	userGraphQL     = `query($org: String!, $email: String!) {
	 organization(login: $org) {
	   samlIdentityProvider {
		 externalIdentities(first: 1, userName: $email) {
		   nodes {
			 user {
			   url
			   login
			   name
			 }
		   }
		 }
	   }
	 }
	}`
	allUserGraphQL = `query FetchUsersWithEmail($org: String!, $cursor: String) {
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

func (c Client) GetUserByEmail(email string) (*User, error) {
	bearerToken, err := c.createBearerToken()
	if err != nil {
		return nil, fmt.Errorf("creating bearer token: %v", err)
	}

	query := map[string]any{
		"query": userGraphQL,
		"variables": map[string]string{
			"org":   c.org,
			"email": email,
		},
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshalling query: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, graphqlEndpoint, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", bearerToken))
	req.Header.Add("Content-Type", "application/json")

	httpClient := http.Client{
		Timeout: 10 * time.Second,
	}

	httpResp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("doing request: %v", err)
	}
	defer httpResp.Body.Close()

	var githubResp githubResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&githubResp); err != nil {
		return nil, fmt.Errorf("decoding response: %v", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error fetching user (%v): %s", httpResp.Status, githubResp.Errors)
	}

	if len(githubResp.Errors) > 0 {
		var b strings.Builder
		for _, err := range githubResp.Errors {
			fmt.Fprintf(&b, "%s (type=%s, path=[%s])\n", err.Message, err.Type, strings.Join(err.Path, " "))
		}

		return nil, fmt.Errorf("graphql error: %s", b.String())
	}

	if len(githubResp.Data.Organization.SamlIdentityProvider.ExternalIdentities.Nodes) == 0 {
		return nil, nil
	}

	user := githubResp.Data.Organization.SamlIdentityProvider.ExternalIdentities.Nodes[0].User
	return &User{
		Login: user.Login,
	}, nil
}
