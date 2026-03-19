-- +goose Up
ALTER TABLE slack_messages DROP CONSTRAINT slack_messages_pkey;
ALTER TABLE slack_messages ADD PRIMARY KEY (team_slug, event_id, channel);

-- +goose Down
ALTER TABLE slack_messages DROP CONSTRAINT slack_messages_pkey;
ALTER TABLE slack_messages ADD PRIMARY KEY (team_slug, event_id);
