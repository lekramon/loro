-- +goose Up
CREATE TABLE source_configuration (
 id INTEGER PRIMARY KEY CHECK(id=1),
 revision INTEGER NOT NULL CHECK(revision>=1),
 name TEXT NOT NULL,
 endpoint TEXT NOT NULL,
 api_key TEXT NOT NULL,
 capabilities TEXT CHECK(capabilities IS NULL OR json_valid(capabilities)),
 checked_at TEXT
);

-- +goose Down
DROP TABLE source_configuration;
