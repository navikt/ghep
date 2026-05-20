-- name: GetDigestSentAt :one
SELECT sent_at FROM digest_sent WHERE team_slug = $1;

-- name: UpsertDigestSent :exec
INSERT INTO digest_sent (team_slug, sent_at) VALUES ($1, $2)
ON CONFLICT (team_slug) DO UPDATE SET sent_at = EXCLUDED.sent_at;
