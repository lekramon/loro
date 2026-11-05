package httpapi

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lekramon/piratarr/internal/library"
	"github.com/lekramon/piratarr/internal/settings"
	"github.com/lekramon/piratarr/internal/storage"
)

func TestLibraryInventoryRepeatMissingRestartAndPreservation(t *testing.T) {
	data := t.TempDir()
	store, err := storage.Open(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { store.Close() }()
	input := validSetup(t)
	h := New(store, fstest.MapFS{}, "127.0.0.1:8080")
	call := func(method, path, body string) library.Snapshot {
		t.Helper()
		r := setupRequest(t, h, method, path, []byte(body), false)
		return decodeResponse[library.Snapshot](t, r, 200)
	}
	if r := setupRequest(t, h, "POST", "/api/v1/library/scan", []byte(`{}`), false); r.Code != 409 {
		t.Fatal(r.Code, r.Body.String())
	}
	if r := setupRequest(t, h, "POST", "/api/v1/setup", encoded(t, input), false); r.Code != 201 {
		t.Fatal(r.Code, r.Body.String())
	}
	empty := call("GET", "/api/v1/library/files", "")
	if empty.Scan != nil || empty.Total != 0 {
		t.Fatal(empty)
	}
	root := input.Directories.MovieDirectory
	os.Mkdir(filepath.Join(root, "Nested"), 0700)
	file := filepath.Join(root, "Nested", "Movie.MKV")
	payload := []byte("unchanged original media")
	if err = os.WriteFile(file, payload, 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(file)
	os.WriteFile(filepath.Join(root, "empty.mp4"), nil, 0600)
	os.WriteFile(filepath.Join(root, "note.txt"), []byte("notes"), 0600)
	scanned := call("POST", "/api/v1/library/scan", `{}`)
	if scanned.Total != 2 || scanned.Scan.FilesPresent != 2 || scanned.Scan.SkippedEntries != 1 {
		t.Fatal(scanned)
	}
	var id int64
	for _, entry := range scanned.Items {
		if entry.RelativePath == "Nested/Movie.MKV" {
			id = entry.ID
			if entry.Size != int64(len(payload)) {
				t.Fatal(entry)
			}
		}
	}
	if id == 0 {
		t.Fatal(scanned)
	}
	repeated := call("POST", "/api/v1/library/scan", `{}`)
	if repeated.Total != 2 {
		t.Fatal(repeated)
	}
	after, _ := os.Stat(file)
	bytes, _ := os.ReadFile(file)
	if string(bytes) != string(payload) || !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("media changed")
	}
	if err = os.Remove(file); err != nil {
		t.Fatal(err)
	}
	missing := call("POST", "/api/v1/library/scan", `{}`)
	if missing.Scan.FilesPresent != 1 {
		t.Fatal(missing)
	}
	filtered := call("GET", "/api/v1/library/files?status=missing&limit=1&offset=0", "")
	if filtered.Total != 1 || filtered.Items[0].ID != id || filtered.Items[0].Status != "missing" {
		t.Fatal(filtered)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = storage.Open(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	h = New(store, fstest.MapFS{}, "127.0.0.1:8080")
	persisted := call("GET", "/api/v1/library/files?status=missing", "")
	if persisted.Total != 1 || persisted.Items[0].ID != id {
		t.Fatal(persisted)
	}
	os.WriteFile(file, payload, 0600)
	returned := call("POST", "/api/v1/library/scan", `{}`)
	for _, entry := range returned.Items {
		if entry.ID == id && entry.Status != "present" {
			t.Fatal(entry)
		}
	}
	renamed := root + "-offline"
	if err = os.Rename(root, renamed); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(renamed, root)
	failed := setupRequest(t, h, "POST", "/api/v1/library/scan", []byte(`{}`), false)
	if failed.Code != 503 || strings.Contains(failed.Body.String(), root) {
		t.Fatal(failed.Code, failed.Body.String())
	}
	unchanged := call("GET", "/api/v1/library/files", "")
	if unchanged.Scan.ScannedAt != returned.Scan.ScannedAt || unchanged.Scan.FilesPresent != 2 {
		t.Fatal(unchanged)
	}
}
func TestLibraryRejectsSymlinksAndInvalidRequests(t *testing.T) {
	f := newCatalogFixture(t, true)
	config, err := f.store.GetSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	root := config.Directories.MovieDirectory
	os.WriteFile(filepath.Join(root, "local.mkv"), []byte("local"), 0600)
	outside := filepath.Join(t.TempDir(), "outside.mkv")
	os.WriteFile(outside, []byte("outside"), 0600)
	// Windows may need developer mode for symlink creation; Linux must exercise it.
	linked := os.Symlink(outside, filepath.Join(root, "escape.mkv")) == nil
	if !linked && runtime.GOOS != "windows" {
		t.Fatal("Linux symlink fixture unavailable")
	}
	if linked {
		os.Symlink(root, filepath.Join(root, "loop"))
	}
	r := f.request(t, "POST", "/api/v1/library/scan", `{}`)
	snapshot := decodeResponse[library.Snapshot](t, r, 200)
	if snapshot.Total != 1 {
		t.Fatal(snapshot)
	}
	for _, body := range []string{`null`, `[]`, `{"root":"elsewhere"}`, `{} {}`, ``} {
		if r = f.request(t, "POST", "/api/v1/library/scan", body); r.Code != 400 {
			t.Fatal(body, r.Code)
		}
	}
	for _, query := range []string{"?status=bad", "?limit=0", "?offset=-1", "?limit=1&limit=2", "?unknown=1"} {
		if r = f.request(t, "GET", "/api/v1/library/files"+query, ""); r.Code != 422 {
			t.Fatal(query, r.Code)
		}
	}
	if r = f.request(t, "POST", "/api/v1/library/scan?path=elsewhere", `{}`); r.Code != 422 {
		t.Fatal(r.Code)
	}
}

type changedScanStore struct {
	*storage.Store
	input settings.Input
}

func (s *changedScanStore) PublishInventory(ctx context.Context, root string, revision int, files []library.Observation, skipped int) error {
	if _, err := s.Store.UpdateSettings(ctx, revision, s.input); err != nil {
		return err
	}
	return s.Store.PublishInventory(ctx, root, revision, files, skipped)
}
func TestLibrarySettingsRaceNeverPublishes(t *testing.T) {
	f := newCatalogFixture(t, true)
	config, err := f.store.GetSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	h := New(&changedScanStore{Store: f.store, input: settings.Input{Directories: config.Directories, Profile: config.Profile}}, fstest.MapFS{}, "127.0.0.1:8080")
	r := setupRequest(t, h, "POST", "/api/v1/library/scan", []byte(`{}`), false)
	if r.Code != 409 || !strings.Contains(r.Body.String(), "settings-changed") {
		t.Fatal(r.Code, r.Body.String())
	}
	result := decodeResponse[library.Snapshot](t, f.request(t, "GET", "/api/v1/library/files", ""), 200)
	if result.Scan != nil {
		t.Fatal(result)
	}
}

func TestLibraryUnreadableDirectoryPreservesSnapshot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission semantics; Windows covered by missing-root and native scan tests")
	}
	f := newCatalogFixture(t, true)
	config, err := f.store.GetSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	root := config.Directories.MovieDirectory
	blocked := filepath.Join(root, "blocked")
	if err = os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(blocked, "file.mkv"), []byte("original"), 0600)
	first := decodeResponse[library.Snapshot](t, f.request(t, "POST", "/api/v1/library/scan", `{}`), 200)
	if err = os.Chmod(blocked, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(blocked, 0700)
	response := f.request(t, "POST", "/api/v1/library/scan", `{}`)
	if response.Code != 503 {
		t.Fatal(response.Code, response.Body.String())
	}
	after := decodeResponse[library.Snapshot](t, f.request(t, "GET", "/api/v1/library/files", ""), 200)
	if after.Scan.ScannedAt != first.Scan.ScannedAt || after.Items[0].Status != "present" {
		t.Fatal(after)
	}
}
func TestLibraryRootChangeRetainsIndependentInventory(t *testing.T) {
	f := newCatalogFixture(t, true)
	ctx := context.Background()
	config, err := f.store.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(config.Directories.MovieDirectory, "original.mkv"), []byte("original"), 0600)
	original := decodeResponse[library.Snapshot](t, f.request(t, "POST", "/api/v1/library/scan", `{}`), 200)
	replacement := settings.Input{Directories: config.Directories, Profile: config.Profile}
	replacement.Directories.MovieDirectory = t.TempDir()
	changed, err := f.store.UpdateSettings(ctx, config.Revision, replacement)
	if err != nil {
		t.Fatal(err)
	}
	empty := decodeResponse[library.Snapshot](t, f.request(t, "GET", "/api/v1/library/files", ""), 200)
	if empty.Total != 0 || empty.Scan != nil {
		t.Fatal(empty)
	}
	if _, err = f.store.UpdateSettings(ctx, changed.Revision, settings.Input{Directories: config.Directories, Profile: config.Profile}); err != nil {
		t.Fatal(err)
	}
	restored := decodeResponse[library.Snapshot](t, f.request(t, "GET", "/api/v1/library/files", ""), 200)
	if restored.Total != 1 || restored.Items[0].ID != original.Items[0].ID {
		t.Fatal(restored)
	}
}
