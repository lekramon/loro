-- +goose Up
CREATE TABLE source_configuration_multiple (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 active INTEGER NOT NULL DEFAULT 0 CHECK(active IN (0,1)),
 revision INTEGER NOT NULL CHECK(revision>=1),
 name TEXT NOT NULL,
 endpoint TEXT NOT NULL,
 api_key TEXT NOT NULL,
 capabilities TEXT CHECK(capabilities IS NULL OR json_valid(capabilities)),
 checked_at TEXT,
 kind TEXT NOT NULL DEFAULT 'torznab' CHECK(kind IN ('torznab','1337x'))
);

INSERT INTO source_configuration_multiple(id,active,revision,name,endpoint,api_key,capabilities,checked_at,kind)
SELECT id,1,revision,name,endpoint,api_key,capabilities,checked_at,kind FROM source_configuration;

DROP TABLE source_configuration;
ALTER TABLE source_configuration_multiple RENAME TO source_configuration;
CREATE UNIQUE INDEX source_configuration_active ON source_configuration(active) WHERE active=1;
CREATE UNIQUE INDEX source_configuration_endpoint ON source_configuration(endpoint);

-- +goose Down
SELECT 1; -- Downgrades require restoring a matching stopped-service backup.
