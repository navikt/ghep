package github

import "log/slog"

type Userer interface {
	GetUserByEmail(email string) (User, error)
}

type Client struct {
	log               *slog.Logger
	apiURL            string
	appInstallationID string
	appID             string
	appPrivateKey     string
	org               string
}

func New(log *slog.Logger, githubAPI, appInstallationID, appID, appPrivateKey, githubOrg string) Client {
	return Client{
		log:               log,
		apiURL:            githubAPI,
		appInstallationID: appInstallationID,
		appID:             appID,
		appPrivateKey:     appPrivateKey,
		org:               githubOrg,
	}
}
