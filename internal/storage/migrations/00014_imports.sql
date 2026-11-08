-- +goose Up
CREATE TABLE import_jobs (
    id TEXT PRIMARY KEY,
    download_id TEXT NOT NULL REFERENCES download_tasks(acquisition_id),
    movie_id INTEGER NOT NULL REFERENCES movies(id),
    source_path TEXT NOT NULL,
    destination_path TEXT NOT NULL,
    relative_path TEXT NOT NULL,
    state TEXT NOT NULL CHECK(state IN ('planned','staging','published','committed','needs-review','ignored')),
    phase TEXT NOT NULL,
    revision INTEGER NOT NULL CHECK(revision >= 1),
    error_code TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX import_jobs_updated ON import_jobs(updated_at,id);

-- +goose Down
DROP TABLE import_jobs;
