-- +goose Up
CREATE TABLE security_digest_sent (
    team_slug TEXT PRIMARY KEY REFERENCES teams(slug) ON DELETE CASCADE,
    sent_at   TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE security_digest_sent;
