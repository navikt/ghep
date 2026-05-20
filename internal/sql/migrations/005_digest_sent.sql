-- +goose Up
CREATE TABLE digest_sent (
    team_slug TEXT PRIMARY KEY,
    sent_at   TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE digest_sent;
