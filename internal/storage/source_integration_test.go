package storage

import (
	"context"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/library"
	"github.com/lekramon/piratarr/internal/sources"
)

func TestSourceMigrationPreservesV5AndWriteFailure(t *testing.T) {
	ctx := context.Background()
	data := t.TempDir()
	v5 := fstest.MapFS{}
	for _, name := range []string{"00001_foundation.sql", "00002_quality_formats.sql", "00003_catalog.sql", "00004_library_inventory.sql", "00005_file_review.sql"} {
		bytes, err := fs.ReadFile(migrations, "migrations/"+name)
		if err != nil {
			t.Fatal(err)
		}
		v5["migrations/"+name] = &fstest.MapFile{Data: bytes}
	}
	old, err := open(ctx, data, v5)
	if err != nil {
		t.Fatal(err)
	}
	config, err := old.CreateSettings(ctx, libraryInput(t))
	if err != nil {
		t.Fatal(err)
	}
	movie, _, err := old.RegisterMovie(ctx, catalog.Metadata{TMDBID: 42, Title: "Original title", Genres: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	root := config.Directories.MovieDirectory
	if err = old.PublishInventory(ctx, root, config.Revision, []library.Observation{{RelativePath: "movie.mkv", Size: 20, ModifiedAt: time.Now().UTC().Format(time.RFC3339Nano)}}, 0); err != nil {
		t.Fatal(err)
	}
	inventory, err := old.ListInventory(ctx, root, 50, 0, "all")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := old.CommitReview(ctx, root, config.Revision, inventory.Items[0].ID, library.Review{Revision: 1, Action: "associate", MovieID: &movie.ID, ConfirmIdentity: true})
	if err != nil {
		t.Fatal(err)
	}
	old.Close()
	store, err := Open(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	saved, err := store.GetSettings(ctx)
	if err != nil || !reflect.DeepEqual(saved, config) {
		t.Fatal("settings changed", err)
	}
	file, err := store.InventoryFile(ctx, root, entry.ID)
	if err != nil || !reflect.DeepEqual(file, entry) {
		t.Fatal("association changed", err)
	}
	title, err := store.MovieByID(ctx, movie.ID)
	if err != nil || !title.HasMedia {
		t.Fatal(title, err)
	}
	initial, err := store.GetSource(ctx)
	if err != nil || initial.Configured || initial.Revision != 0 {
		t.Fatal(initial.Public, err)
	}
	if _, err = store.write.Exec(`CREATE TRIGGER source_failure BEFORE INSERT ON source_configuration BEGIN SELECT RAISE(ABORT,'private SQL failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveSource(ctx, 0, "Source", "http://localhost/api", "dummy-secret"); err == nil {
		t.Fatal("write failure ignored")
	}
	unchanged, err := store.GetSource(ctx)
	if err != nil || unchanged.Configured {
		t.Fatal(unchanged.Public, err)
	}
	if _, err = store.write.Exec("DROP TRIGGER source_failure"); err != nil {
		t.Fatal(err)
	}
	source, err := store.SaveSource(ctx, 0, "Source", "http://localhost/api", "dummy-secret")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SaveSourceCapabilities(ctx, source.Revision+1, sources.Capabilities{}, "now"); err != sources.ErrChanged {
		t.Fatal(err)
	}
	store.Close()
	if rejected, err := open(ctx, data, v5); err == nil {
		rejected.Close()
		t.Fatal("old binary accepted v6")
	}
	recovered, err := Open(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	if _, err = recovered.write.Exec("DROP TABLE source_configuration"); err != nil {
		t.Fatal(err)
	}
	if recovered.Ready(ctx) == nil {
		t.Fatal("missing source table considered ready")
	}
}
