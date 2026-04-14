CREATE TABLE IF NOT EXISTS webhooks (
    id                TEXT PRIMARY KEY,
    url               TEXT NOT NULL,
    secret            TEXT NOT NULL DEFAULT '',
    event_types       TEXT NOT NULL DEFAULT '[]',
    enabled           INTEGER NOT NULL DEFAULT 1,
    last_delivered_at TEXT,
    last_error        TEXT,
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_webhooks_enabled ON webhooks(enabled);