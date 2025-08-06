-- name: CreateUser :exec
INSERT INTO users (login) VALUES ($1)
ON CONFLICT (login) DO NOTHING;

-- name: ExistsUser :one
SELECT EXISTS (SELECT login FROM users WHERE login = $1);

-- name: DeleteUser :exec
DELETE FROM users WHERE login = $1;

-- name: CreateEmail :exec
INSERT INTO emails (login, email) VALUES ($1, $2)
ON CONFLICT (login, email) DO NOTHING;

-- name: GetUserByEmail :one
SELECT e.login
FROM emails e
WHERE e.email ilike $1;
