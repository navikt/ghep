-- name: CreateSlackMessage :exec
INSERT INTO slack_messages (team_slug, event_id, thread_ts, channel, payload) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (team_slug, event_id) DO UPDATE
SET thread_ts = EXCLUDED.thread_ts,
    channel = EXCLUDED.channel,
    payload = EXCLUDED.payload;

-- name: UpdateSlackMessage :exec
UPDATE slack_messages
SET payload = $3
WHERE team_slug = $1 AND event_id = $2;

-- name: GetSlackMessage :one
SELECT thread_ts, channel, payload
FROM slack_messages
WHERE team_slug = $1 AND event_id = $2;

-- name: CreateSlackID :exec
INSERT INTO slack_ids (login, id) VALUES ($1, $2)
ON CONFLICT (login, id) DO NOTHING;

-- name: GetUserSlackID :one
SELECT id
FROM slack_ids
WHERE login = $1;
