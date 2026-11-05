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
	"github.com/lekramon/piratarr/internal/settings"
)

func libraryInput(t *testing.T) settings.Input {
	return settings.Input{Directories: settings.Directories{DownloadDirectory: t.TempDir(), MovieDirectory: t.TempDir()}, Profile: settings.Profile{AcceptedResolutions: []string{"1080p"}, PreferredResolution: "1080p", AcceptedFormats: settings.Formats(), AcceptedContainers: settings.Containers(), AudioPreferences: []string{"original"}}}
}
func TestInventoryMigrationPreservesV3AndRefusesDowngrade(t *testing.T) {
	ctx := context.Background()
	data := t.TempDir()
	v3 := fstest.MapFS{}
	for _, name := range []string{"00001_foundation.sql", "00002_quality_formats.sql", "00003_catalog.sql"} {
		bytes, err := fs.ReadFile(migrations, "migrations/"+name)
		if err != nil {
			t.Fatal(err)
		}
		v3["migrations/"+name] = &fstest.MapFile{Data: bytes}
	}
	old, err := open(ctx, data, v3)
	if err != nil {
		t.Fatal(err)
	}
	before, err := old.CreateSettings(ctx, libraryInput(t))
	if err != nil {
		t.Fatal(err)
	}
	original, _, err := old.RegisterMovie(ctx, catalog.Metadata{TMDBID: 42, Title: "Preserved movie", Genres: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	old.Close()
	upgraded, err := Open(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	after, err := upgraded.GetSettings(ctx)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("settings changed", err)
	}
	movie, err := upgraded.MovieByID(ctx, original.ID)
	if err != nil || !reflect.DeepEqual(movie, original) {
		t.Fatal("catalog changed", err)
	}
	result, err := upgraded.ListInventory(ctx, before.Directories.MovieDirectory, 50, 0, "all")
	if err != nil || result.Scan != nil || result.Total != 0 {
		t.Fatal(result, err)
	}
	upgraded.Close()
	if rejected, err := open(ctx, data, v3); err == nil {
		rejected.Close()
		t.Fatal("older binary accepted v4")
	}
	recovered, err := Open(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	if _, err = recovered.write.Exec("DROP TABLE library_scan_summary"); err != nil {
		t.Fatal(err)
	}
	if recovered.Ready(ctx) == nil {
		t.Fatal("readiness accepted missing inventory table")
	}
}
func TestInventoryPublicationRollsBackOnFailure(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	config, err := store.CreateSettings(ctx, libraryInput(t))
	if err != nil {
		t.Fatal(err)
	}
	root := config.Directories.MovieDirectory
	first := []library.Observation{{RelativePath: "movie.mkv", Size: 12, ModifiedAt: time.Now().UTC().Format(time.RFC3339Nano)}}
	if err = store.PublishInventory(ctx, root, config.Revision, first, 0); err != nil {
		t.Fatal(err)
	}
	before, err := store.ListInventory(ctx, root, 50, 0, "all")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.write.Exec(`CREATE TRIGGER inventory_failure BEFORE INSERT ON library_files BEGIN SELECT RAISE(ABORT,'private injected persistence failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if err = store.PublishInventory(ctx, root, config.Revision, []library.Observation{{RelativePath: "new.mp4", Size: 20, ModifiedAt: first[0].ModifiedAt}}, 0); err == nil {
		t.Fatal("write failure ignored")
	}
	after, err := store.ListInventory(ctx, root, 50, 0, "all")
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("partial inventory published", after, err)
	}
	if _, err = store.write.Exec("DROP TRIGGER inventory_failure"); err != nil {
		t.Fatal(err)
	}
	if err = store.PublishInventory(ctx, root, config.Revision, first, 0); err != nil {
		t.Fatal("retry failed", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if err = store.PublishInventory(ctx, root, config.Revision, nil, 0); err == nil {
		t.Fatal("canceled transaction committed")
	}
}
