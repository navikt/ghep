-- name: ClaimTeamDigestSlot :one
INSERT INTO team_digest_sent (type, team_slug, sent_at)
VALUES (@type, @team_slug, @sent_at)
ON CONFLICT (type, team_slug) DO UPDATE
  SET sent_at = EXCLUDED.sent_at
  WHERE team_digest_sent.sent_at < @scheduled_at
RETURNING sent_at;
