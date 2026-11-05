package storage

import (
	"context"
	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/library"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
	"time"
)

func TestReviewMigrationPreservesInventoryAndAtomicReview(t *testing.T) {
	ctx := context.Background()
	data := t.TempDir()
	v4 := fstest.MapFS{}
	for _, name := range []string{"00001_foundation.sql", "00002_quality_formats.sql", "00003_catalog.sql", "00004_library_inventory.sql"} {
		bytes, err := fs.ReadFile(migrations, "migrations/"+name)
		if err != nil {
			t.Fatal(err)
		}
		v4["migrations/"+name] = &fstest.MapFile{Data: bytes}
	}
	old, err := open(ctx, data, v4)
	if err != nil {
		t.Fatal(err)
	}
	inputSettings := libraryInput(t)
	inputSettings.Directories.MovieDirectory = filepath.Join(inputSettings.Directories.MovieDirectory, "ÉMovies")
	if err = os.Mkdir(inputSettings.Directories.MovieDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	settings, err := old.CreateSettings(ctx, inputSettings)
	if err != nil {
		t.Fatal(err)
	}
	movie, _, err := old.RegisterMovie(ctx, catalog.Metadata{TMDBID: 99, Title: "Preserved title", Genres: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	root := settings.Directories.MovieDirectory
	if err = old.PublishInventory(ctx, root, settings.Revision, []library.Observation{{RelativePath: "Example (2024).mkv", Size: 14, ModifiedAt: time.Now().UTC().Format(time.RFC3339Nano)}}, 0); err != nil {
		t.Fatal(err)
	}
	before, err := old.ListInventory(ctx, root, 50, 0, "all")
	if err != nil {
		t.Fatal(err)
	}
	old.Close()
	store, err := Open(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	after, err := store.ListInventory(ctx, root, 50, 0, "all")
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("inventory changed", after, err)
	}
	saved, err := store.GetSettings(ctx)
	if err != nil || !reflect.DeepEqual(settings, saved) {
		t.Fatal("settings changed", err)
	}
	entry := after.Items[0]
	input := library.Review{Revision: entry.Revision, Action: "associate", MovieID: &movie.ID, ConfirmIdentity: true}
	if _, err = store.write.Exec(`CREATE TRIGGER review_failure BEFORE UPDATE ON library_files BEGIN SELECT RAISE(ABORT,'private SQL fault'); END;`); err != nil {
		t.Fatal(err)
	}
	if _, err = store.CommitReview(ctx, root, settings.Revision, entry.ID, input); err == nil {
		t.Fatal("fault ignored")
	}
	unchanged, err := store.InventoryFile(ctx, root, entry.ID)
	if err != nil || !reflect.DeepEqual(entry, unchanged) {
		t.Fatal("partial review", err)
	}
	if _, err = store.write.Exec("DROP TRIGGER review_failure"); err != nil {
		t.Fatal(err)
	}
	linked, err := store.CommitReview(ctx, root, settings.Revision, entry.ID, input)
	if err != nil || linked.ReviewState != "associated" {
		t.Fatal(linked, err)
	}
	actual, err := store.MovieByID(ctx, movie.ID)
	if err != nil || !actual.HasMedia {
		t.Fatal(actual, err)
	}
	store.Close()
	if rejected, err := open(ctx, data, v4); err == nil {
		rejected.Close()
		t.Fatal("v4 binary accepted v5")
	}
}
