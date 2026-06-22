-- name: UpsertUserCommitCount :exec
INSERT INTO user_commit_counts (login, repo, commit_count, last_pushed_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (login, repo)
DO UPDATE SET
    commit_count   = user_commit_counts.commit_count + EXCLUDED.commit_count,
    last_pushed_at = EXCLUDED.last_pushed_at;

-- name: GetUserCommitsSince :many
SELECT repo, commit_count
FROM user_commit_counts
WHERE login = $1 AND last_pushed_at > $2
ORDER BY commit_count DESC;

-- name: ListUsersWithCommitsSince :many
SELECT DISTINCT login FROM user_commit_counts WHERE last_pushed_at > $1;

-- name: GetPersonalDigestSentAt :one
SELECT sent_at FROM personal_digest_sent WHERE login = $1;

-- name: ClaimPersonalDigestSlot :one
INSERT INTO personal_digest_sent (login, sent_at)
VALUES ($1, $2)
ON CONFLICT (login) DO UPDATE
  SET sent_at = EXCLUDED.sent_at
  WHERE personal_digest_sent.sent_at < $3
RETURNING sent_at;
