package github

import "log/slog"

type Userer interface {
	GetUserByEmail(email string) (User, error)
}

type Client struct {
	log               *slog.Logger
	appInstallationID string
	appID             string
	appPrivateKey     string
	org               string
}

func New(log *slog.Logger, appInstallationID, appID, appPrivateKey, githubOrg string) Client {
	return Client{
		log:               log,
		appInstallationID: appInstallationID,
		appID:             appID,
		appPrivateKey:     appPrivateKey,
		org:               githubOrg,
	}
}
