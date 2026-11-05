package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lekramon/piratarr/internal/httpapi"
	"github.com/lekramon/piratarr/internal/settings"
)

func TestSettingsUpdateFailureRollsBackRevisionAndProfileThroughAPI(t *testing.T) {
	store, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := httpapi.New(store, fstest.MapFS{}, "127.0.0.1:8080")
	request := func(method, path string, value any) *httptest.ResponseRecorder {
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, "http://localhost:8080"+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Piratarr-Request", "1")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	original := settings.Input{Directories: settings.Directories{DownloadDirectory: t.TempDir(), MovieDirectory: t.TempDir()}, Profile: settings.Profile{AcceptedResolutions: []string{"720p", "1080p"}, PreferredResolution: "1080p", AudioPreferences: []string{"pt", "original"}}}
	if response := request("POST", "/api/v1/setup", original); response.Code != 201 {
		t.Fatal(response.Body.String())
	}
	baseline, err := store.GetSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	update := settings.UpdateInput{Revision: 1, Input: settings.Input{Directories: settings.Directories{DownloadDirectory: t.TempDir(), MovieDirectory: t.TempDir()}, Profile: settings.Profile{AcceptedFormats: settings.Formats(), AcceptedContainers: settings.Containers(), AcceptedResolutions: []string{"2160p"}, PreferredResolution: "2160p", AudioPreferences: []string{"en"}}}}
	if _, err := store.write.Exec("CREATE TRIGGER update_failure BEFORE INSERT ON audio_preferences BEGIN SELECT RAISE(ABORT,'controlled update failure'); END;"); err != nil {
		t.Fatal(err)
	}
	response := request("PUT", "/api/v1/settings", update)
	if response.Code != 500 || strings.Contains(response.Body.String(), "controlled update failure") {
		t.Fatalf("fault leaked or wrong status: %d %s", response.Code, response.Body.String())
	}
	after, err := store.GetSettings(context.Background())
	if err != nil || !reflect.DeepEqual(after, baseline) {
		t.Fatalf("partial update after fault: %+v %v", after, err)
	}
	if _, err := store.write.Exec("DROP TRIGGER update_failure; PRAGMA query_only=ON"); err != nil {
		t.Fatal(err)
	}
	if response := request("PUT", "/api/v1/settings", update); response.Code != 503 {
		t.Fatalf("read-only update: %d %s", response.Code, response.Body.String())
	}
	after, err = store.GetSettings(context.Background())
	if err != nil || !reflect.DeepEqual(after, baseline) {
		t.Fatal("read-only failure changed settings")
	}
	if _, err := store.write.Exec("PRAGMA query_only=OFF"); err != nil {
		t.Fatal(err)
	}
	if response := request("PUT", "/api/v1/settings", update); response.Code != 200 {
		t.Fatalf("retry after recovery: %d %s", response.Code, response.Body.String())
	}
	after, err = store.GetSettings(context.Background())
	if err != nil || after.Revision != 2 || !reflect.DeepEqual(after.Profile, update.Profile) {
		t.Fatalf("failed retries incorrectly advanced revision: %+v %v", after, err)
	}
}
