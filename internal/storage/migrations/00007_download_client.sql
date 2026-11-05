-- +goose Up
CREATE TABLE download_client_configuration (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    revision INTEGER NOT NULL CHECK (revision >= 1),
    endpoint TEXT NOT NULL,
    username TEXT NOT NULL,
    password TEXT NOT NULL,
    app_version TEXT,
    api_version TEXT,
    checked_at TEXT
);

-- +goose Down
DROP TABLE download_client_configuration;
