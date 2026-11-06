-- +goose Up
CREATE TABLE submission_jobs (
 acquisition_id TEXT PRIMARY KEY REFERENCES acquisitions(id),
 info_hash TEXT NOT NULL UNIQUE,
 kind TEXT NOT NULL DEFAULT 'submit' CHECK(kind='submit'),
 payload_version INTEGER NOT NULL DEFAULT 1 CHECK(payload_version=1),
 client_revision INTEGER NOT NULL,
 settings_revision INTEGER NOT NULL,
 download_directory TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN('queued','running','succeeded','needs-review')),
 due_at INTEGER NOT NULL,
 attempts INTEGER NOT NULL DEFAULT 0,
 lease_token TEXT,
 lease_expires INTEGER,
 add_attempted INTEGER NOT NULL DEFAULT 0 CHECK(add_attempted IN(0,1)),
 error_code TEXT,
 updated_at TEXT NOT NULL
);
CREATE INDEX submission_due ON submission_jobs(state,due_at);
-- +goose Down
DROP TABLE submission_jobs;
