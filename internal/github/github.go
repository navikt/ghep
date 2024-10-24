package github

type Userer interface {
	GetUserByEmail(email string) (User, error)
}

type Client struct {
	apiURL            string
	appInstallationID string
	appID             string
	appPrivateKey     string
	org               string
}

func New(githubAPI, appInstallationID, appID, appPrivateKey, githubOrg string) Client {
	return Client{
		apiURL:            githubAPI,
		appInstallationID: appInstallationID,
		appID:             appID,
		appPrivateKey:     appPrivateKey,
		org:               githubOrg,
	}
}
