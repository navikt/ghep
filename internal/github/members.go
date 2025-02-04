package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// fetchMembers fetches members for a Github org or team using the following API:
// Team: https://docs.github.com/en/rest/teams/members#list-team-members
// Org: https://docs.github.com/en/rest/orgs/members#list-organization-members
func fetchMembers(teamURL, bearerToken string) ([]User, error) {
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

	var teamMembers []User
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

		var members []User
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

func (c *Client) FetchOrgMembers() ([]User, error) {
	bearerToken, err := c.createBearerToken()
	if err != nil {
		return []User{}, fmt.Errorf("creating bearer token: %v", err)
	}

	url := fmt.Sprintf("https://api.github.com/orgs/%s", c.org)
	return fetchMembers(url, bearerToken)
}
