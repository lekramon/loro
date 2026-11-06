-- +goose Up
CREATE TABLE release_observations (
    id TEXT PRIMARY KEY,
    movie_id INTEGER NOT NULL REFERENCES movies(id),
    source_revision INTEGER NOT NULL,
    profile_revision INTEGER NOT NULL,
    snapshot TEXT NOT NULL CHECK(json_valid(snapshot)),
    source_url TEXT NOT NULL,
    reference TEXT NOT NULL,
    hash_claim TEXT NOT NULL,
    reference_issue TEXT NOT NULL,
    expires_at INTEGER NOT NULL
);
CREATE TABLE acquisitions (
    id TEXT PRIMARY KEY,
    observation_id TEXT NOT NULL REFERENCES release_observations(id),
    movie_id INTEGER NOT NULL REFERENCES movies(id),
    candidate_id TEXT NOT NULL,
    snapshot TEXT NOT NULL CHECK(json_valid(snapshot)),
    state TEXT NOT NULL CHECK(state IN ('selected','preparing','ready','needs-review','cancelled')),
    revision INTEGER NOT NULL CHECK(revision>=1),
    info_hash TEXT,
    payload_kind TEXT,
    review_code TEXT,
    payload BLOB,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX acquisition_active_candidate ON acquisitions(movie_id,candidate_id) WHERE state!='cancelled';
CREATE TABLE acquisition_requests (
    request_key TEXT PRIMARY KEY,
    observation_id TEXT NOT NULL,
    confirm_override INTEGER NOT NULL CHECK(confirm_override IN(0,1)),
    acquisition_id TEXT NOT NULL REFERENCES acquisitions(id)
);

-- +goose Down
DROP TABLE acquisition_requests;
DROP TABLE acquisitions;
DROP TABLE release_observations;
