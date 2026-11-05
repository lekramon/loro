package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/lekramon/piratarr/internal/httpapi"
	"github.com/lekramon/piratarr/internal/settings"
)

func TestSetupTransactionFailuresAreAtomicThroughAPI(t *testing.T) {
	store, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := httpapi.New(store, fstest.MapFS{}, "127.0.0.1:8080")
	input := settings.Input{Directories: settings.Directories{DownloadDirectory: t.TempDir(), MovieDirectory: t.TempDir()}, Profile: settings.Profile{AcceptedResolutions: []string{"1080p"}, PreferredResolution: "1080p", AudioPreferences: []string{"original"}}}
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	post := func() *httptest.ResponseRecorder {
		request := httptest.NewRequest("POST", "http://localhost:8080/api/v1/setup", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Piratarr-Request", "1")
		response := httptest.NewRecorder()
		h.ServeHTTP(response, request)
		return response
	}
	// A real SQLite trigger fails the final table write, after the singleton
	// and resolution inserts, proving that all writes roll back together.
	if _, err := store.write.Exec("CREATE TRIGGER setup_failure BEFORE INSERT ON audio_preferences BEGIN SELECT RAISE(ABORT,'controlled setup failure'); END;"); err != nil {
		t.Fatal(err)
	}
	if response := post(); response.Code != 500 {
		t.Fatalf("unexpected fault: %d %s", response.Code, response.Body.String())
	}
	for _, table := range []string{"settings", "accepted_resolutions", "audio_preferences"} {
		var count int
		if err := store.read.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("partial writes in %s: %d %v", table, count, err)
		}
	}
	if _, err := store.write.Exec("DROP TRIGGER setup_failure; PRAGMA query_only=ON"); err != nil {
		t.Fatal(err)
	}
	if response := post(); response.Code != 503 {
		t.Fatalf("read-only storage should be unavailable: %d %s", response.Code, response.Body.String())
	}
	if _, err := store.write.Exec("PRAGMA query_only=OFF"); err != nil {
		t.Fatal(err)
	}
	if response := post(); response.Code != 201 {
		t.Fatalf("recovery: %d %s", response.Code, response.Body.String())
	}
}
