-- +goose Up
ALTER TABLE library_files ADD COLUMN revision INTEGER NOT NULL DEFAULT 1;
ALTER TABLE library_files ADD COLUMN review_state TEXT NOT NULL DEFAULT 'pending' CHECK(review_state IN ('pending','associated','ignored'));
ALTER TABLE library_files ADD COLUMN movie_id INTEGER REFERENCES movies(id);
ALTER TABLE library_files ADD COLUMN verified_at TEXT;
CREATE INDEX library_files_movie ON library_files(movie_id,present,review_state);

-- +goose Down
DROP INDEX library_files_movie;
ALTER TABLE library_files DROP COLUMN verified_at;
ALTER TABLE library_files DROP COLUMN movie_id;
ALTER TABLE library_files DROP COLUMN review_state;
ALTER TABLE library_files DROP COLUMN revision;
