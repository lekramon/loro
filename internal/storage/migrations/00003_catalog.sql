-- +goose Up
CREATE TABLE movies (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 tmdb_id INTEGER NOT NULL UNIQUE CHECK(tmdb_id>0),
 title TEXT NOT NULL,
 metadata TEXT NOT NULL CHECK(json_valid(metadata)),
 added_at TEXT NOT NULL
);
CREATE INDEX movies_title ON movies(title COLLATE NOCASE,id);
CREATE TABLE metadata_cache (
 cache_key TEXT PRIMARY KEY,
 payload BLOB NOT NULL,
 fetched_at TEXT NOT NULL
);
CREATE TABLE metadata_artwork (
 local_url TEXT PRIMARY KEY
);

-- +goose Down
DROP TABLE metadata_artwork;
DROP TABLE metadata_cache;
DROP TABLE movies;
