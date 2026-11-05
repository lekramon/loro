-- +goose Up
CREATE TABLE settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    revision INTEGER NOT NULL CHECK (revision >= 1),
    download_directory TEXT NOT NULL,
    movie_directory TEXT NOT NULL,
    preferred_resolution TEXT NOT NULL CHECK (preferred_resolution IN ('480p','576p','720p','1080p','2160p')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE accepted_resolutions (
    settings_id INTEGER NOT NULL REFERENCES settings(id) ON DELETE CASCADE,
    resolution TEXT NOT NULL CHECK (resolution IN ('480p','576p','720p','1080p','2160p')),
    PRIMARY KEY (settings_id, resolution)
);
CREATE TABLE audio_preferences (
    settings_id INTEGER NOT NULL REFERENCES settings(id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position >= 0),
    choice TEXT NOT NULL,
    PRIMARY KEY (settings_id, position),
    UNIQUE (settings_id, choice)
);

-- +goose Down
DROP TABLE audio_preferences;
DROP TABLE accepted_resolutions;
DROP TABLE settings;
