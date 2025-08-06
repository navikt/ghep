package api

import (
	"context"
	"log/slog"
	"testing"

	"github.com/navikt/ghep/internal/events"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/sql/gensql"
	"github.com/pashagolub/pgxmock/v4"
)

func TestIsAnExternalContributorEvent(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery(gensql.ExistsUser).
		WithArgs("InternalUser").
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("EXISTS").
		WithArgs("ExternalUser").
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

	db := gensql.New(mock)
	apiClient := New(slog.Default(), db, events.Handler{}, map[string]github.Team{}, "externalChannel", false)

	event := github.Event{
		Sender: github.User{
			Login: "InternalUser",
			Type:  "User",
		},
	}
	isExternal, err := apiClient.isAnExternalContributorEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if isExternal {
		t.Errorf("expected isExternal to be false, got true")
	}

	event.Sender.Login = "ExternalUser"
	isExternal, err = apiClient.isAnExternalContributorEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !isExternal {
		t.Errorf("expected isExternal to be true, got false")
	}

	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
