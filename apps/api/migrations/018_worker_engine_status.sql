-- Store effective conversion engine availability reported by each worker.

ALTER TABLE worker_status ADD COLUMN engines_json TEXT NOT NULL DEFAULT '{}';
