-- Runtime control switches for admin operational actions.

CREATE TABLE IF NOT EXISTS runtime_controls (
    id                 INTEGER PRIMARY KEY CHECK (id = 1),
    job_intake_paused  INTEGER NOT NULL DEFAULT 0,
    pause_reason       TEXT,
    updated_by         TEXT,
    updated_at         TEXT NOT NULL
);

INSERT INTO runtime_controls (id, job_intake_paused, pause_reason, updated_by, updated_at)
SELECT 1, 0, NULL, NULL, strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE NOT EXISTS (SELECT 1 FROM runtime_controls WHERE id = 1);
