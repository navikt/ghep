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
WHERE login ILIKE $1 AND last_pushed_at > $2
ORDER BY commit_count DESC;

-- name: ResetUserCommitCounts :exec
UPDATE user_commit_counts SET commit_count = 0 WHERE login ILIKE $1;

-- name: UpsertWorkflowFailure :exec
INSERT INTO user_workflow_failures (login, repo, failure_count, last_failed_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (login, repo)
DO UPDATE SET
    failure_count  = user_workflow_failures.failure_count + EXCLUDED.failure_count,
    last_failed_at = EXCLUDED.last_failed_at;

-- name: GetWorkflowFailuresSince :many
SELECT repo, failure_count
FROM user_workflow_failures
WHERE login ILIKE $1 AND last_failed_at > $2
ORDER BY failure_count DESC;

-- name: ResetWorkflowFailures :exec
UPDATE user_workflow_failures SET failure_count = 0 WHERE login ILIKE $1;

-- name: ListUsersWithCommitsSince :many
SELECT DISTINCT login FROM user_commit_counts WHERE last_pushed_at > $1;

-- name: GetPersonalDigestSentAt :one
SELECT sent_at FROM personal_digest_sent WHERE login = $1;

-- name: ClaimPersonalDigestSlot :one
INSERT INTO personal_digest_sent (login, sent_at)
VALUES (@login, @sent_at)
ON CONFLICT (login) DO UPDATE
  SET sent_at = EXCLUDED.sent_at
  WHERE personal_digest_sent.sent_at < @scheduled_at
RETURNING sent_at;
