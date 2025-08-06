-- +goose Up
CREATE TABLE slack_ids (
    login text NOT NULL,
    id text NOT NULL UNIQUE,
    PRIMARY KEY (login, id),
    FOREIGN KEY (login) REFERENCES users(login) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE slack_ids;
