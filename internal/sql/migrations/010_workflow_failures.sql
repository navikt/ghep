-- +goose Up
CREATE TABLE user_workflow_failures (
    login          TEXT        NOT NULL REFERENCES users(login) ON DELETE CASCADE,
    repo           TEXT        NOT NULL,
    failure_count  INT         NOT NULL DEFAULT 0,
    last_failed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (login, repo)
);

-- +goose Down
DROP TABLE user_workflow_failures;
