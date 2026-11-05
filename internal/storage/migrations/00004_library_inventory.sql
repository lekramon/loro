-- +goose Up
CREATE TABLE library_files (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 root_key TEXT NOT NULL,
 path_key TEXT NOT NULL,
 relative_path TEXT NOT NULL,
 size INTEGER NOT NULL CHECK(size>=0),
 modified_at TEXT NOT NULL,
 present INTEGER NOT NULL CHECK(present IN (0,1)),
 first_seen_at TEXT NOT NULL,
 last_seen_at TEXT NOT NULL,
 UNIQUE(root_key,path_key)
);
CREATE INDEX library_files_root ON library_files(root_key,present,relative_path,id);
CREATE TABLE library_scan_summary (
 root_key TEXT PRIMARY KEY,
 scanned_at TEXT NOT NULL,
 files_present INTEGER NOT NULL,
 skipped_entries INTEGER NOT NULL
);

-- +goose Down
DROP TABLE library_scan_summary;
DROP TABLE library_files;
