package github

import (
	"log/slog"

	"github.com/navikt/ghep/internal/sql/gensql"
)

type Userer interface {
	GetUserByEmail(email string) (*User, error)
}

type Client struct {
	log               *slog.Logger
	db                *gensql.Queries
	appInstallationID string
	appID             string
	appPrivateKey     string
	org               string
}

func New(log *slog.Logger, db *gensql.Queries, appInstallationID, appID, appPrivateKey, githubOrg string) Client {
	return Client{
		log:               log,
		db:                db,
		appInstallationID: appInstallationID,
		appID:             appID,
		appPrivateKey:     appPrivateKey,
		org:               githubOrg,
	}
}
