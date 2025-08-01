-- +goose Up
CREATE table slack_messages (
    team_slug text NOT NULL,
    event_id text NOT NULL,
    thread_ts text NOT NULL,
    channel text NOT NULL,
    payload json NOT NULL,
    PRIMARY KEY (team_slug, event_id),

    FOREIGN KEY (team_slug) REFERENCES teams(slug) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE slack_messages;
