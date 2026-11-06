-- +goose Up
CREATE TABLE download_tasks (
 acquisition_id TEXT PRIMARY KEY REFERENCES submission_jobs(acquisition_id),
 kind TEXT NOT NULL DEFAULT 'poll' CHECK(kind='poll'),
 payload_version INTEGER NOT NULL DEFAULT 1 CHECK(payload_version=1),
 revision INTEGER NOT NULL DEFAULT 1 CHECK(revision>=1),
 state TEXT NOT NULL DEFAULT 'pending' CHECK(state IN('pending','queued','downloading','paused','unavailable','completed','failed','needs-review')),
 snapshot TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(snapshot)),
 error_code TEXT,
 checked_at TEXT,
 observed_at TEXT,
 completed_at TEXT,
 handoff TEXT CHECK(handoff IS NULL OR json_valid(handoff)),
 poll_state TEXT NOT NULL DEFAULT 'queued' CHECK(poll_state IN('queued','running','idle')),
 due_at INTEGER,
 attempts INTEGER NOT NULL DEFAULT 0,
 lease_token TEXT,
 lease_expires INTEGER,
 updated_at TEXT NOT NULL
);
CREATE INDEX download_due ON download_tasks(poll_state,due_at);
CREATE TABLE download_events (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 acquisition_id TEXT NOT NULL REFERENCES download_tasks(acquisition_id),
 state TEXT NOT NULL,
 code TEXT,
 progress REAL,
 handoff_state TEXT,
 created_at TEXT NOT NULL
);
CREATE INDEX download_history ON download_events(acquisition_id,id);
INSERT INTO download_tasks(acquisition_id,due_at,updated_at)
 SELECT acquisition_id,CAST(strftime('%s','now') AS INTEGER)*1000,updated_at FROM submission_jobs WHERE state='succeeded';
INSERT INTO download_events(acquisition_id,state,created_at)
 SELECT acquisition_id,state,updated_at FROM download_tasks;
-- +goose Down
DROP TABLE download_events;
DROP TABLE download_tasks;
