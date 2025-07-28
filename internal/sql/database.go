package sql

import (
	"context"
	"embed"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
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
	url = fmt.Sprintf("%s?default_query_exec_mode=cache_describe", url)

	if err := goose.SetDialect("postgres"); err != nil {
		return nil, err
	}

	db, err := goose.OpenDBWithDriver("pgx", url)
	if err != nil {
		return nil, fmt.Errorf("failed to open database with driver: %w", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return gensql.New(pool), nil
}
