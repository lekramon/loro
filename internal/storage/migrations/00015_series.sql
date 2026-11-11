-- +goose Up
ALTER TABLE movies ADD COLUMN anime INTEGER NOT NULL DEFAULT 0 CHECK(anime IN (0,1));

CREATE TABLE series (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider TEXT NOT NULL CHECK(provider IN ('tmdb')),
    provider_id INTEGER NOT NULL CHECK(provider_id>0),
    title TEXT NOT NULL,
    metadata TEXT NOT NULL CHECK(json_valid(metadata)),
    anime INTEGER NOT NULL DEFAULT 0 CHECK(anime IN (0,1)),
    added_at TEXT NOT NULL,
    UNIQUE(provider, provider_id)
);
CREATE INDEX series_title ON series(title COLLATE NOCASE,id);

CREATE TABLE seasons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    series_id INTEGER NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    season_number INTEGER NOT NULL CHECK(season_number>=0),
    title TEXT,
    metadata TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(metadata)),
    UNIQUE(series_id, season_number)
);

CREATE TABLE episodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    season_id INTEGER NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    provider_id INTEGER,
    episode_number INTEGER CHECK(episode_number IS NULL OR episode_number>0),
    absolute_number INTEGER CHECK(absolute_number IS NULL OR absolute_number>0),
    title TEXT NOT NULL,
    metadata TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(metadata)),
    UNIQUE(season_id, episode_number),
    CHECK(episode_number IS NOT NULL OR absolute_number IS NOT NULL)
);
CREATE INDEX episodes_season ON episodes(season_id,episode_number,id);

CREATE TABLE episode_library_files (
    episode_id INTEGER NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    library_file_id INTEGER NOT NULL UNIQUE REFERENCES library_files(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL DEFAULT 1 CHECK(revision>=1),
    verified_at TEXT,
    PRIMARY KEY(episode_id, library_file_id)
);

-- +goose Down
DROP TABLE episode_library_files;
DROP TABLE episodes;
DROP TABLE seasons;
DROP INDEX series_title;
DROP TABLE series;
ALTER TABLE movies DROP COLUMN anime;
