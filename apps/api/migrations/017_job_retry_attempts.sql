-- Persist retry lineage so capability MaxRetries can be enforced.

ALTER TABLE jobs ADD COLUMN source_job_id TEXT;
ALTER TABLE jobs ADD COLUMN attempt_number INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_jobs_source_job_id ON jobs(source_job_id);
