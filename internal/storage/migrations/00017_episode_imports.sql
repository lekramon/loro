-- +goose Up
-- Expand import identity from movie-only to exactly one movie or episode.
-- Existing movie import rows retain their identifiers unchanged.
CREATE TABLE import_jobs_017 (
    id TEXT PRIMARY KEY,
    download_id TEXT NOT NULL REFERENCES download_tasks(acquisition_id),
    movie_id INTEGER REFERENCES movies(id),
    episode_id INTEGER REFERENCES episodes(id),
    source_path TEXT NOT NULL,
    destination_path TEXT NOT NULL,
    relative_path TEXT NOT NULL,
    state TEXT NOT NULL CHECK(state IN ('planned','staging','published','committed','needs-review','ignored')),
    phase TEXT NOT NULL,
    revision INTEGER NOT NULL CHECK(revision >= 1),
    error_code TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK ((movie_id IS NOT NULL) != (episode_id IS NOT NULL))
);
INSERT INTO import_jobs_017
    SELECT id,download_id,movie_id,NULL,source_path,destination_path,relative_path,state,phase,revision,error_code,created_at,updated_at
    FROM import_jobs;
DROP INDEX import_jobs_updated;
DROP TABLE import_jobs;
ALTER TABLE import_jobs_017 RENAME TO import_jobs;
CREATE INDEX import_jobs_updated ON import_jobs(updated_at,id);

-- +goose Down
-- Down migrations are intentionally unsupported after episode imports exist.
SELECT RAISE(ABORT, 'episode import migration requires restore from a schema-16 backup');
