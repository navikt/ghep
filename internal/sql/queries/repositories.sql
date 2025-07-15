-- name: CreateRepository :one
INSERT INTO repositories (name) VALUES ($1)
ON CONFLICT (name) DO NOTHING
RETURNING id;

-- name: GetRepository :one
SELECT id, name FROM repositories WHERE name = $1;

-- name: UpdateRepository :exec
UPDATE repositories
SET name = @name
WHERE name = @old_name;
