package storage

import (
	"context"
	"testing"
)

func TestSeriesMigrationIsAdditiveAndPreservesMovies(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil { t.Fatal(err) }
	defer store.Close()
	if _, err = store.write.ExecContext(ctx, `INSERT INTO movies(tmdb_id,title,metadata,added_at) VALUES(999001,'Existing movie','{}','now')`); err != nil { t.Fatal(err) }
	var title string
	if err = store.read.QueryRowContext(ctx, "SELECT title FROM movies WHERE tmdb_id=999001").Scan(&title); err != nil || title != "Existing movie" { t.Fatalf("movie preservation: %q %v", title, err) }
	var tables int
	if err = store.read.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('series','seasons','episodes','episode_library_files')").Scan(&tables); err != nil || tables != 4 { t.Fatalf("series schema missing: %d %v", tables, err) }
	var anime int
	if err = store.read.QueryRowContext(ctx, "SELECT anime FROM movies WHERE tmdb_id=999001").Scan(&anime); err != nil || anime != 0 { t.Fatalf("movie anime default: %d %v", anime, err) }
	for _, table := range []string{"release_observations", "acquisitions"} {
		var columns int
		if err = store.read.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info(?) WHERE name='episode_id'", table).Scan(&columns); err != nil || columns != 1 {
			t.Fatalf("episode identity missing from %s: %d %v", table, columns, err)
		}
	}
}
