-- 001_initial.sql
-- Foundation tables for Reform Lab file conversion service (SQLite).

CREATE TABLE IF NOT EXISTS files (
    id                 TEXT PRIMARY KEY,
    internal_name      TEXT    NOT NULL,
    original_name      TEXT    NOT NULL,
    size               INTEGER NOT NULL,
    mime_type          TEXT    NOT NULL,
    format_family      TEXT    NOT NULL,
    detected_extension TEXT    NOT NULL,
    metadata           TEXT    NOT NULL DEFAULT '{}',
    uploaded_at        TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS jobs (
    id              TEXT PRIMARY KEY,
    file_id         TEXT    NOT NULL REFERENCES files(id),
    capability_id   TEXT    NOT NULL,
    output_format   TEXT    NOT NULL,
    status          TEXT    NOT NULL DEFAULT 'queued',
    progress        INTEGER NOT NULL DEFAULT 0,
    error           TEXT,
    artifact_id     TEXT,
    started_at      TEXT,
    completed_at    TEXT,
    created_at      TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_jobs_status  ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_file_id ON jobs(file_id);

CREATE TABLE IF NOT EXISTS artifacts (
    id           TEXT PRIMARY KEY,
    job_id       TEXT    NOT NULL REFERENCES jobs(id),
    file_id      TEXT    NOT NULL REFERENCES files(id),
    file_name    TEXT    NOT NULL,
    mime_type    TEXT    NOT NULL,
    size         INTEGER NOT NULL,
    storage_path TEXT    NOT NULL,
    created_at   TEXT    NOT NULL,
    expires_at   TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_artifacts_job_id ON artifacts(job_id);

CREATE TABLE IF NOT EXISTS audit_events (
    id         TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    file_id    TEXT,
    job_id     TEXT,
    details    TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_events_file_id    ON audit_events(file_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_event_type ON audit_events(event_type);
