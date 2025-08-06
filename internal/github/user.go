package github

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
