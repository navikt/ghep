-- name: ClaimSecurityDigestSlot :one
INSERT INTO security_digest_sent (team_slug, sent_at)
VALUES ($1, $2)
ON CONFLICT (team_slug) DO UPDATE
  SET sent_at = EXCLUDED.sent_at
  WHERE security_digest_sent.sent_at < $3
RETURNING sent_at;
