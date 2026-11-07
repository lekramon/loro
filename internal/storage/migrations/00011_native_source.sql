-- +goose Up
ALTER TABLE source_configuration ADD COLUMN kind TEXT NOT NULL DEFAULT 'torznab' CHECK(kind IN ('torznab','1337x'));

-- +goose Down
SELECT 1; -- Downgrades require restoring a matching stopped-service backup.
