ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user';
ALTER TABLE files ADD COLUMN user_id TEXT;
ALTER TABLE jobs ADD COLUMN user_id TEXT;
ALTER TABLE artifacts ADD COLUMN user_id TEXT;

CREATE INDEX IF NOT EXISTS idx_files_user_id ON files(user_id);
CREATE INDEX IF NOT EXISTS idx_jobs_user_id ON jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_user_id ON artifacts(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_single_admin ON users(role) WHERE role = 'admin';

UPDATE users SET role = 'user' WHERE role IS NULL OR role = '';
UPDATE users
SET role = 'admin'
WHERE id = (
  SELECT id
  FROM users
  ORDER BY created_at ASC
  LIMIT 1
)
AND NOT EXISTS (
  SELECT 1
  FROM users
  WHERE role = 'admin'
);

UPDATE jobs
SET user_id = (
  SELECT files.user_id
  FROM files
  WHERE files.id = jobs.file_id
)
WHERE user_id IS NULL;

UPDATE artifacts
SET user_id = (
  SELECT jobs.user_id
  FROM jobs
  WHERE jobs.id = artifacts.job_id
)
WHERE user_id IS NULL;