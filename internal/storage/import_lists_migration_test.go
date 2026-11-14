package storage

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestImportListMigrationPreservesExistingCatalog(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	legacy := fstest.MapFS{}
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == "00018_import_lists.sql" {
			continue
		}
		data, readErr := fs.ReadFile(migrations, "migrations/"+entry.Name())
		if readErr != nil {
			t.Fatal(readErr)
		}
		legacy["migrations/"+entry.Name()] = &fstest.MapFile{Data: data}
	}
	store, err := open(ctx, directory, legacy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.write.ExecContext(ctx, `INSERT INTO movies(id,tmdb_id,title,metadata,added_at) VALUES(77,77,'Preserved','{}','before')`); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, directory)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var title string
	if err = store.read.QueryRowContext(ctx, `SELECT title FROM movies WHERE id=77 AND tmdb_id=77`).Scan(&title); err != nil || title != "Preserved" {
		t.Fatal(title, err)
	}
	for _, table := range []string{"import_list_previews", "import_list_preview_rows", "import_lists", "import_list_items"} {
		var count int
		if err = store.read.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil || count != 1 {
			t.Fatal(table, count, err)
		}
	}
}
