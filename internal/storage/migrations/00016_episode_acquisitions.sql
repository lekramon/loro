-- +goose Up
-- Expand the acquisition identity from movie-only to exactly one movie or episode.
-- Existing rows are copied unchanged with episode_id NULL.
PRAGMA foreign_keys=OFF;

ALTER TABLE download_events RENAME TO download_events_015;
ALTER TABLE download_tasks RENAME TO download_tasks_015;
ALTER TABLE submission_jobs RENAME TO submission_jobs_015;
ALTER TABLE acquisition_requests RENAME TO acquisition_requests_015;
ALTER TABLE acquisitions RENAME TO acquisitions_015;
ALTER TABLE release_observations RENAME TO release_observations_015;

DROP INDEX acquisition_active_candidate;
DROP INDEX submission_due;
DROP INDEX download_due;
DROP INDEX download_history;

CREATE TABLE release_observations (
    id TEXT PRIMARY KEY,
    movie_id INTEGER REFERENCES movies(id),
    episode_id INTEGER REFERENCES episodes(id),
    source_revision INTEGER NOT NULL,
    profile_revision INTEGER NOT NULL,
    snapshot TEXT NOT NULL CHECK(json_valid(snapshot)),
    source_url TEXT NOT NULL,
    reference TEXT NOT NULL,
    hash_claim TEXT NOT NULL,
    reference_issue TEXT NOT NULL,
    expires_at INTEGER NOT NULL,
    CHECK ((movie_id IS NOT NULL) != (episode_id IS NOT NULL))
);
INSERT INTO release_observations
 SELECT id,movie_id,NULL,source_revision,profile_revision,snapshot,source_url,reference,hash_claim,reference_issue,expires_at
 FROM release_observations_015;

CREATE TABLE acquisitions (
    id TEXT PRIMARY KEY,
    observation_id TEXT NOT NULL REFERENCES release_observations(id),
    movie_id INTEGER REFERENCES movies(id),
    episode_id INTEGER REFERENCES episodes(id),
    candidate_id TEXT NOT NULL,
    snapshot TEXT NOT NULL CHECK(json_valid(snapshot)),
    state TEXT NOT NULL CHECK(state IN ('selected','preparing','ready','needs-review','cancelled')),
    revision INTEGER NOT NULL CHECK(revision>=1),
    info_hash TEXT,
    payload_kind TEXT,
    review_code TEXT,
    payload BLOB,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK ((movie_id IS NOT NULL) != (episode_id IS NOT NULL))
);
INSERT INTO acquisitions
 SELECT id,observation_id,movie_id,NULL,candidate_id,snapshot,state,revision,info_hash,payload_kind,review_code,payload,created_at,updated_at
 FROM acquisitions_015;
CREATE UNIQUE INDEX acquisition_active_candidate ON acquisitions(movie_id,episode_id,candidate_id) WHERE state!='cancelled';

CREATE TABLE acquisition_requests (
    request_key TEXT PRIMARY KEY,
    observation_id TEXT NOT NULL,
    confirm_override INTEGER NOT NULL CHECK(confirm_override IN(0,1)),
    acquisition_id TEXT NOT NULL REFERENCES acquisitions(id)
);
INSERT INTO acquisition_requests SELECT * FROM acquisition_requests_015;

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
INSERT INTO submission_jobs SELECT * FROM submission_jobs_015;
CREATE INDEX submission_due ON submission_jobs(state,due_at);

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
INSERT INTO download_tasks SELECT * FROM download_tasks_015;
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
INSERT INTO download_events SELECT * FROM download_events_015;
CREATE INDEX download_history ON download_events(acquisition_id,id);

DROP TABLE download_events_015;
DROP TABLE download_tasks_015;
DROP TABLE submission_jobs_015;
DROP TABLE acquisition_requests_015;
DROP TABLE acquisitions_015;
DROP TABLE release_observations_015;
PRAGMA foreign_keys=ON;

-- +goose Down
-- Down migrations are intentionally unsupported after episode rows exist.
SELECT RAISE(ABORT, 'episode acquisition migration requires restore from a schema-15 backup');
