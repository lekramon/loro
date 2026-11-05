package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/httpapi"
	"github.com/lekramon/piratarr/internal/settings"
	"github.com/lekramon/piratarr/internal/tmdb"
)

func TestCatalogPersistenceFailureIsAtomicThroughAPI(t *testing.T) {
	store, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	input := settings.Input{Directories: settings.Directories{DownloadDirectory: t.TempDir(), MovieDirectory: t.TempDir()}, Profile: settings.Profile{AcceptedResolutions: []string{"1080p"}, PreferredResolution: "1080p", AudioPreferences: []string{"original"}}}
	if _, err = store.CreateSettings(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	before, err := store.GetSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"id":42,"title":"One movie","original_title":"One movie","original_language":"en","release_date":"2024-01-01"}`))
	}))
	defer provider.Close()
	client := tmdb.New(tmdb.Config{Token: func() string { return "dummy-token" }, APIBase: provider.URL})
	handler := httpapi.New(store, fstest.MapFS{}, "127.0.0.1:8080", catalog.New(store, client, store.DataDirectory()))
	register := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "http://localhost:8080/api/v1/catalog/movies", bytes.NewBufferString(`{"tmdbId":42}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Piratarr-Request", "1")
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, req)
		return result
	}
	if _, err = store.write.Exec(`CREATE TRIGGER catalog_failure BEFORE INSERT ON movies BEGIN SELECT RAISE(ABORT,'do not expose this private injected error'); END;`); err != nil {
		t.Fatal(err)
	}
	response := register()
	if response.Code != 500 || strings.Contains(response.Body.String(), "injected") {
		t.Fatal(response.Code, response.Body.String())
	}
	records, err := store.ListMovies(context.Background(), 50, 0, "")
	if err != nil || records.Total != 0 {
		t.Fatal("partial movie persisted", records, err)
	}
	if _, err = store.write.Exec("DROP TRIGGER catalog_failure; PRAGMA query_only=ON"); err != nil {
		t.Fatal(err)
	}
	if response = register(); response.Code != 503 {
		t.Fatal(response.Code, response.Body.String())
	}
	if _, err = store.write.Exec("PRAGMA query_only=OFF"); err != nil {
		t.Fatal(err)
	}
	if response = register(); response.Code != 201 {
		t.Fatal(response.Code, response.Body.String())
	}
	var saved struct {
		Movie catalog.Movie `json:"movie"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if response = register(); response.Code != 200 {
		t.Fatal(response.Code)
	}
	if calls.Load() != 1 {
		t.Fatal("retry discarded cached authoritative metadata")
	}
	after, err := store.GetSettings(context.Background())
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("catalog write modified setup", err)
	}
}

func TestCatalogMigrationPreservesVersionTwoSettings(t *testing.T) {
	one, err := migrations.ReadFile("migrations/00001_foundation.sql")
	if err != nil {
		t.Fatal(err)
	}
	two, err := migrations.ReadFile("migrations/00002_quality_formats.sql")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	old, err := open(context.Background(), directory, fstest.MapFS{"migrations/00001_foundation.sql": {Data: one}, "migrations/00002_quality_formats.sql": {Data: two}})
	if err != nil {
		t.Fatal(err)
	}
	input := settings.Input{Directories: settings.Directories{DownloadDirectory: t.TempDir(), MovieDirectory: t.TempDir()}, Profile: settings.Profile{AcceptedResolutions: []string{"1080p"}, PreferredResolution: "1080p", AudioPreferences: []string{"pt"}, AcceptedFormats: []string{"remux"}, AcceptedContainers: []string{"mkv"}}}
	if _, err = old.CreateSettings(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	before, err := old.GetSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	old.Close()
	upgraded, err := Open(context.Background(), directory)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	after, err := upgraded.GetSettings(context.Background())
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("migration changed settings", err)
	}
	empty, err := upgraded.ListMovies(context.Background(), 50, 0, "")
	if err != nil || empty.Total != 0 {
		t.Fatal(empty, err)
	}
	if _, err = upgraded.write.Exec("DROP TABLE metadata_cache"); err != nil {
		t.Fatal(err)
	}
	if upgraded.Ready(context.Background()) == nil {
		t.Fatal("readiness ignored damaged catalog schema")
	}
}
