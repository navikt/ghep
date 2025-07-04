-- name: CreateTeam :exec
INSERT INTO teams (slug) VALUES ($1)
ON CONFLICT (slug) DO NOTHING;

-- name: ListTeams :many
SELECT slug FROM teams ORDER BY slug;

-- name: ListTeamsByRepository :many
SELECT t.slug
FROM teams t
JOIN team_repositories tr ON t.slug = tr.team_slug
JOIN repositories r ON tr.repository_id = r.id
WHERE r.name = $1
ORDER BY t.slug;

-- name: GetTeam :one
SELECT slug FROM teams WHERE slug = $1;

-- name: AddTeamMember :exec
INSERT INTO team_members (team_slug, user_login) VALUES ($1, $2)
ON CONFLICT (team_slug, user_login) DO NOTHING;

-- name: ListTeamMembers :many
SELECT user_login FROM team_members WHERE team_slug = $1 ORDER BY user_login;

-- name: GetTeamMember :one
SELECT user_login FROM team_members WHERE team_slug = $1 AND user_login = $2;

-- name: RemoveTeamMember :exec
DELETE FROM team_members WHERE team_slug = $1 AND user_login = $2;

-- name: GetTeamMemberByEmail :one
SELECT tm.user_login
FROM team_members tm
JOIN emails e ON tm.user_login = e.login
WHERE tm.team_slug = $1 AND e.email = $2;

-- name: GetTeamMembersWithEmails :many
SELECT tm.user_login, e.email
FROM team_members tm
JOIN emails e ON tm.user_login = e.login
WHERE tm.team_slug = $1
ORDER BY tm.user_login;

-- name: AddTeamRepository :exec
INSERT INTO team_repositories (team_slug, repository_id) VALUES ($1, $2)
ON CONFLICT (team_slug, repository_id) DO NOTHING;

-- name: RemoveTeamRepository :exec
DELETE FROM team_repositories
WHERE team_slug = $1 AND repository_id = (SELECT id FROM repositories WHERE name = $2);

-- name: ListTeamRepositories :many
SELECT r.id, r.name
FROM team_repositories tr
JOIN repositories r ON tr.repository_id = r.id
WHERE tr.team_slug = $1
ORDER BY r.name;
