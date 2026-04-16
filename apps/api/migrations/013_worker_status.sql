-- Worker runtime status for admin health visibility.

CREATE TABLE IF NOT EXISTS worker_status (
    id                    TEXT PRIMARY KEY,
    runtime_mode          TEXT NOT NULL,
    queue_mode            TEXT NOT NULL,
    last_heartbeat_at     TEXT NOT NULL,
    last_task_type        TEXT,
    last_job_id           TEXT,
    last_task_status      TEXT NOT NULL DEFAULT 'idle',
    last_task_started_at  TEXT,
    last_task_finished_at TEXT,
    last_error            TEXT,
    updated_at            TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS worker_failures (
    id         TEXT PRIMARY KEY,
    worker_id  TEXT NOT NULL,
    task_type  TEXT,
    job_id     TEXT,
    error      TEXT NOT NULL,
    failed_at  TEXT NOT NULL,
    FOREIGN KEY(worker_id) REFERENCES worker_status(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_worker_failures_worker_time
    ON worker_failures(worker_id, failed_at DESC);
