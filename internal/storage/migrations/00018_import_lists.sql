-- +goose Up
CREATE TABLE import_list_previews (
    id TEXT PRIMARY KEY,
    source_kind TEXT NOT NULL CHECK(source_kind IN ('letterboxd-csv','imdb-csv')),
    list_key TEXT NOT NULL,
    name TEXT NOT NULL,
    language TEXT NOT NULL CHECK(language IN ('en-US','pt-BR')),
    input_hash TEXT NOT NULL,
    state TEXT NOT NULL CHECK(state IN ('preview','committed')),
    revision INTEGER NOT NULL DEFAULT 1 CHECK(revision>=1),
    created_at TEXT NOT NULL,
    committed_at TEXT,
    list_id INTEGER,
    linked_count INTEGER NOT NULL DEFAULT 0,
    created_count INTEGER NOT NULL DEFAULT 0,
    duplicate_count INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE import_list_preview_rows (
    preview_id TEXT NOT NULL REFERENCES import_list_previews(id) ON DELETE CASCADE,
    row_number INTEGER NOT NULL CHECK(row_number>0),
    source_item_key TEXT NOT NULL,
    title TEXT NOT NULL,
    year INTEGER,
    tmdb_id INTEGER,
    status TEXT NOT NULL CHECK(status IN ('addition','duplicate','ambiguous','missing','invalid')),
    error_code TEXT,
    PRIMARY KEY(preview_id,row_number)
);
CREATE TABLE import_lists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_kind TEXT NOT NULL CHECK(source_kind IN ('letterboxd-csv','imdb-csv')),
    list_key TEXT NOT NULL,
    name TEXT NOT NULL,
    revision INTEGER NOT NULL DEFAULT 1 CHECK(revision>=1),
    updated_at TEXT NOT NULL,
    UNIQUE(source_kind,list_key)
);
CREATE TABLE import_list_items (
    list_id INTEGER NOT NULL REFERENCES import_lists(id) ON DELETE CASCADE,
    source_item_key TEXT NOT NULL,
    title TEXT NOT NULL,
    year INTEGER,
    tmdb_id INTEGER NOT NULL,
    movie_id INTEGER NOT NULL REFERENCES movies(id),
    first_seen_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    PRIMARY KEY(list_id,source_item_key)
);
CREATE INDEX import_list_items_movie ON import_list_items(movie_id,list_id);

-- +goose Down
SELECT RAISE(ABORT, 'import list migration requires restore from a schema-17 backup');
