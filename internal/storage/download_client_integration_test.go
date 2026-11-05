package storage

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/downloadclient"
	"github.com/lekramon/piratarr/internal/library"
	"github.com/lekramon/piratarr/internal/sources"
)

func TestDownloadClientMigrationPreservesV6AndRejectsOldReaders(t *testing.T) {
	ctx := context.Background()
	data := t.TempDir()
	v6 := fstest.MapFS{}
	for _, name := range []string{"00001_foundation.sql", "00002_quality_formats.sql", "00003_catalog.sql", "00004_library_inventory.sql", "00005_file_review.sql", "00006_source.sql"} {
		content, err := fs.ReadFile(migrations, "migrations/"+name)
		if err != nil {
			t.Fatal(err)
		}
		v6["migrations/"+name] = &fstest.MapFile{Data: content}
	}
	old, err := open(ctx, data, v6)
	if err != nil {
		t.Fatal(err)
	}
	config, err := old.CreateSettings(ctx, libraryInput(t))
	if err != nil {
		t.Fatal(err)
	}
	movie, _, err := old.RegisterMovie(ctx, catalog.Metadata{TMDBID: 42, Title: "Preserved movie", Genres: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	root := config.Directories.MovieDirectory
	if err = old.PublishInventory(ctx, root, config.Revision, []library.Observation{{RelativePath: "preserved.mkv", Size: 20, ModifiedAt: time.Now().UTC().Format(time.RFC3339Nano)}}, 0); err != nil {
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
	source, err := old.SaveSource(ctx, 0, "Preserved source", "http://localhost/api", "private-source-key")
	if err != nil {
		t.Fatal(err)
	}
	caps := sources.Capabilities{Search: sources.SearchCapability{Available: true, SupportedParams: []string{"q"}}, Categories: []sources.Category{}}
	if err = old.SaveSourceCapabilities(ctx, source.Revision, caps, "2026-11-05T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	source, err = old.GetSource(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(filepath.Join(data, "credentials"), 0700); err != nil {
		t.Fatal(err)
	}
	token := filepath.Join(data, "credentials", "tmdb-read-token")
	if err = os.WriteFile(token, []byte("dummy-preserved-tmdb-token"), 0600); err != nil {
		t.Fatal(err)
	}
	old.Close()
	// A failed additive migration must leave v6 readable, without a partial table.
	faulty := fstest.MapFS{}
	for name, content := range v6 {
		faulty[name] = content
	}
	faulty["migrations/00007_download_client.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE download_client_configuration(id INTEGER);\nINSERT INTO missing_table VALUES(1);\n")}
	if failed, err := open(ctx, data, faulty); err == nil {
		failed.Close()
		t.Fatal("migration failure ignored")
	}
	old, err = open(ctx, data, v6)
	if err != nil {
		t.Fatal("failed migration broke v6", err)
	}
	var tables int
	if err = old.read.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='download_client_configuration'").Scan(&tables); err != nil || tables != 0 {
		t.Fatal("partial migration persisted", err)
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
		t.Fatal("catalog link lost", err)
	}
	after, err := store.GetSource(ctx)
	if err != nil || !reflect.DeepEqual(after, source) {
		t.Fatal("source configuration changed", err)
	}
	credential, err := os.ReadFile(token)
	if err != nil || string(credential) != "dummy-preserved-tmdb-token" {
		t.Fatal("private credential changed", err)
	}
	initial, err := store.GetDownloadClient(ctx)
	if err != nil || initial.Configured || initial.Revision != 0 {
		t.Fatal(initial.Public, err)
	}
	if _, err = store.write.Exec(`CREATE TRIGGER client_write_failure BEFORE INSERT ON download_client_configuration BEGIN SELECT RAISE(ABORT,'private SQL failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveDownloadClient(ctx, 0, "http://localhost:8081", "admin", "dummy-password"); err != downloadclient.ErrUnavailable {
		t.Fatal("write failure ignored", err)
	}
	initial, err = store.GetDownloadClient(ctx)
	if err != nil || initial.Configured {
		t.Fatal("partial configuration persisted", err)
	}
	if _, err = store.write.Exec("DROP TRIGGER client_write_failure"); err != nil {
		t.Fatal(err)
	}
	client, err := store.SaveDownloadClient(ctx, 0, "http://localhost:8081", "admin", "dummy-password")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SaveDownloadClientTest(ctx, client.Revision+1, "v5.1.0", "2.11.3", "now"); err != downloadclient.ErrChanged {
		t.Fatal("stale evidence persisted", err)
	}
	if _, err = store.SaveDownloadClient(ctx, 0, "http://localhost:8081", "admin", "other"); err != downloadclient.ErrChanged {
		t.Fatal("stale save persisted", err)
	}
	store.Close()
	if rejected, err := open(ctx, data, v6); err == nil {
		rejected.Close()
		t.Fatal("old reader accepted v7")
	}
	recovered, err := Open(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	if _, err = recovered.write.Exec("ALTER TABLE download_client_configuration DROP COLUMN api_version"); err != nil {
		t.Fatal(err)
	}
	if recovered.Ready(ctx) == nil {
		t.Fatal("damaged client table considered ready")
	}
}
