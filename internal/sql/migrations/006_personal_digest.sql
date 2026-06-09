-- +goose Up
CREATE TABLE user_commit_counts (
    login          TEXT        NOT NULL REFERENCES users(login) ON DELETE CASCADE,
    repo           TEXT        NOT NULL,
    commit_count   INT         NOT NULL DEFAULT 0,
    last_pushed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (login, repo)
);

CREATE TABLE personal_digest_sent (
    login   TEXT PRIMARY KEY REFERENCES users(login) ON DELETE CASCADE,
    sent_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE personal_digest_sent;
DROP TABLE user_commit_counts;
