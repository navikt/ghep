-- +goose Up
CREATE TABLE team_digest_sent (
    type      TEXT        NOT NULL CHECK (type IN ('pr', 'security')),
    team_slug TEXT        NOT NULL REFERENCES teams(slug) ON DELETE CASCADE,
    sent_at   TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (type, team_slug)
);

-- Migrate existing rows
INSERT INTO team_digest_sent (type, team_slug, sent_at)
    SELECT 'pr', team_slug, sent_at FROM digest_sent
    ON CONFLICT DO NOTHING;

INSERT INTO team_digest_sent (type, team_slug, sent_at)
    SELECT 'security', team_slug, sent_at FROM security_digest_sent
    ON CONFLICT DO NOTHING;

DROP TABLE digest_sent;
DROP TABLE security_digest_sent;

-- +goose Down
CREATE TABLE digest_sent (
    team_slug TEXT PRIMARY KEY REFERENCES teams(slug) ON DELETE CASCADE,
    sent_at   TIMESTAMPTZ NOT NULL
);

CREATE TABLE security_digest_sent (
    team_slug TEXT PRIMARY KEY REFERENCES teams(slug) ON DELETE CASCADE,
    sent_at   TIMESTAMPTZ NOT NULL
);

INSERT INTO digest_sent (team_slug, sent_at)
    SELECT team_slug, sent_at FROM team_digest_sent WHERE type = 'pr';

INSERT INTO security_digest_sent (team_slug, sent_at)
    SELECT team_slug, sent_at FROM team_digest_sent WHERE type = 'security';

DROP TABLE team_digest_sent;
