-- +goose Up
CREATE table teams (
    slug text PRIMARY KEY
);

CREATE TABLE users (
    login text PRIMARY KEY
);

CREATE TABLE team_members (
    team_slug text NOT NULL,
    user_login text NOT NULL,
    PRIMARY KEY (team_slug, user_login),
    FOREIGN KEY (team_slug) REFERENCES teams(slug) ON DELETE CASCADE,
    FOREIGN KEY (user_login) REFERENCES users(login) ON DELETE CASCADE
);

CREATE TABLE emails (
    login text NOT NULL,
    email text NOT NULL UNIQUE,
    PRIMARY KEY (login, email),
    FOREIGN KEY (login) REFERENCES users(login) ON DELETE CASCADE
);

CREATE TABLE repositories (
    id serial PRIMARY KEY NOT NULL,
    name text NOT NULL UNIQUE
);

CREATE TABLE team_repositories (
    team_slug text NOT NULL,
    repository_id int NOT NULL,
    PRIMARY KEY (team_slug, repository_id),
    FOREIGN KEY (team_slug) REFERENCES teams(slug) ON DELETE CASCADE,
    FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE teams;
DROP TABLE users;
DROP TABLE team_members;
DROP TABLE emails;
DROP TABLE repositories;
