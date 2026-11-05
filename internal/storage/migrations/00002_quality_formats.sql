-- +goose Up
ALTER TABLE settings ADD COLUMN accepted_formats TEXT NOT NULL DEFAULT '["remux","bluray","web-dl","webrip","hdtv","dvd"]';
ALTER TABLE settings ADD COLUMN accepted_containers TEXT NOT NULL DEFAULT '["mkv","mp4","avi","mov","m4v","ts"]';

-- +goose Down
ALTER TABLE settings DROP COLUMN accepted_containers;
ALTER TABLE settings DROP COLUMN accepted_formats;
