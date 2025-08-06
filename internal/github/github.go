package github

import (
	"github.com/navikt/ghep/internal/sql/gensql"
)

type Client struct {
	db                *gensql.Queries
	appInstallationID string
	appID             string
	appPrivateKey     string
	org               string
}

func New(db *gensql.Queries, appInstallationID, appID, appPrivateKey, githubOrg string) Client {
	return Client{
		db:                db,
		appInstallationID: appInstallationID,
		appID:             appID,
		appPrivateKey:     appPrivateKey,
		org:               githubOrg,
	}
}
