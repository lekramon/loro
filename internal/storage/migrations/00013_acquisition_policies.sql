-- +goose Up
CREATE TABLE acquisition_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    revision INTEGER NOT NULL CHECK (revision >= 1),
    accepted_resolutions TEXT NOT NULL CHECK (json_valid(accepted_resolutions)),
    preferred_resolution TEXT NOT NULL,
    accepted_formats TEXT NOT NULL CHECK (json_valid(accepted_formats)),
    accepted_containers TEXT NOT NULL CHECK (json_valid(accepted_containers)),
    audio_preferences TEXT NOT NULL CHECK (json_valid(audio_preferences)),
    cutoff_score INTEGER NOT NULL DEFAULT 0 CHECK (cutoff_score >= 0 AND cutoff_score <= 10000),
    allow_upgrades INTEGER NOT NULL DEFAULT 0 CHECK (allow_upgrades IN (0,1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE quality_definitions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    revision INTEGER NOT NULL CHECK (revision >= 1),
    resolution TEXT NOT NULL,
    source TEXT NOT NULL,
    min_size_per_minute REAL,
    preferred_size_per_minute REAL,
    max_size_per_minute REAL
);
CREATE TABLE custom_formats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    revision INTEGER NOT NULL CHECK (revision >= 1),
    predicates TEXT NOT NULL CHECK (json_valid(predicates)),
    score INTEGER NOT NULL CHECK (score >= -10000 AND score <= 10000),
    minimum_score INTEGER NOT NULL DEFAULT 0 CHECK (minimum_score >= -10000 AND minimum_score <= 10000)
);
CREATE TABLE movie_acquisition_profiles (
    movie_id INTEGER PRIMARY KEY REFERENCES movies(id) ON DELETE CASCADE,
    profile_id INTEGER NOT NULL REFERENCES acquisition_profiles(id),
    assigned_at TEXT NOT NULL
);
INSERT INTO acquisition_profiles(name,revision,accepted_resolutions,preferred_resolution,accepted_formats,accepted_containers,audio_preferences,created_at,updated_at)
SELECT 'Default', revision, (SELECT json_group_array(resolution) FROM (SELECT resolution FROM accepted_resolutions WHERE settings_id=1 ORDER BY CASE resolution WHEN '480p' THEN 1 WHEN '576p' THEN 2 WHEN '720p' THEN 3 WHEN '1080p' THEN 4 WHEN '2160p' THEN 5 END)), preferred_resolution, accepted_formats, accepted_containers, (SELECT json_group_array(choice) FROM (SELECT choice FROM audio_preferences WHERE settings_id=1 ORDER BY position)), created_at, updated_at FROM settings WHERE id=1;

-- +goose Down
DROP TABLE movie_acquisition_profiles;
DROP TABLE custom_formats;
DROP TABLE quality_definitions;
DROP TABLE acquisition_profiles;
