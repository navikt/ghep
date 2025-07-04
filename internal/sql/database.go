package sql

import (
	"context"
	"embed"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/navikt/ghep/internal/sql/gensql"
	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type gooseLogger struct {
	log *slog.Logger
}

func (g *gooseLogger) Fatalf(format string, v ...any) {
	g.log.Error(fmt.Sprintf(format, v...))
}

func (g *gooseLogger) Printf(format string, v ...any) {
	g.log.Info(fmt.Sprintf(format, v...))
}

func New(ctx context.Context, log *slog.Logger, url string) (*gensql.Queries, error) {
	goose.SetBaseFS(embedMigrations)
	goose.SetLogger(&gooseLogger{log: log})

	if err := goose.SetDialect("postgres"); err != nil {
		return nil, err
	}

	db, err := goose.OpenDBWithDriver("pgx", url)
	if err != nil {
		return nil, err
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return nil, err
	}

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return nil, err
	}

	return gensql.New(conn), nil
}

// AddRepositoryToTeam adds a repository to a team, creating the repository if it does not exist.
func AddRepositoryToTeam(ctx context.Context, db *gensql.Queries, team, name string) error {
	repository, err := db.GetRepository(ctx, name)
	if err != nil {
		if err == pgx.ErrNoRows {
			result, err := db.CreateRepository(ctx, gensql.CreateRepositoryParams{
				Name: name,
			})
			if err != nil {
				return err
			}
			repository.ID = result
		}

		return err
	}

	return db.AddTeamRepository(ctx, gensql.AddTeamRepositoryParams{
		TeamSlug:     team,
		RepositoryID: repository.ID,
	})
}

// AddMemberToTeam adds a user to a team, creating the user if it does not exist.
func AddMemberToTeam(ctx context.Context, db *gensql.Queries, team, userLogin string) error {
	if _, err := db.GetUser(ctx, userLogin); err != nil {
		if err == pgx.ErrNoRows {
			if err := db.CreateUser(ctx, userLogin); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return db.AddTeamMember(ctx, gensql.AddTeamMemberParams{
		TeamSlug:  team,
		UserLogin: userLogin,
	})
}
