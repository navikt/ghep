-- +goose Up
-- Remove duplicate rows (stale/deactivated Slack accounts) so one ID per login remains.
-- Active users are re-inserted by FetchSlackUsers on startup.
DELETE FROM slack_ids
WHERE login IN (
    SELECT login FROM slack_ids GROUP BY login HAVING COUNT(*) > 1
);

ALTER TABLE slack_ids DROP CONSTRAINT slack_ids_pkey;
ALTER TABLE slack_ids DROP CONSTRAINT slack_ids_id_key;
ALTER TABLE slack_ids ADD PRIMARY KEY (login);

-- +goose Down
ALTER TABLE slack_ids DROP CONSTRAINT slack_ids_pkey;
ALTER TABLE slack_ids ADD CONSTRAINT slack_ids_id_key UNIQUE (id);
ALTER TABLE slack_ids ADD PRIMARY KEY (login, id);
