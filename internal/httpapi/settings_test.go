package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lekramon/piratarr/internal/settings"
	"github.com/lekramon/piratarr/internal/storage"
)

func configuredAPI(t *testing.T) (*storage.Store, *Handler, settings.Settings) {
	t.Helper()
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	handler := New(store, fstest.MapFS{}, "127.0.0.1:8080")
	response := setupRequest(t, handler, "POST", "/api/v1/setup", encoded(t, validSetup(t)), false)
	if response.Code != 201 {
		t.Fatalf("setup failed: %d %s", response.Code, response.Body.String())
	}
	return store, handler, readSettings(t, handler)
}

func readSettings(t *testing.T, handler *Handler) settings.Settings {
	t.Helper()
	response := setupRequest(t, handler, "GET", "/api/v1/settings", nil, false)
	if response.Code != 200 {
		t.Fatalf("GET settings: %d %s", response.Code, response.Body.String())
	}
	var result settings.Settings
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestSettingsUpdatePersistsAcrossRestartAndPreservesFiles(t *testing.T) {
	store, handler, baseline := configuredAPI(t)
	sentinel := filepath.Join(baseline.Directories.MovieDirectory, "existing.mkv")
	if err := os.WriteFile(sentinel, []byte("existing media"), 0600); err != nil {
		t.Fatal(err)
	}
	next := settings.UpdateInput{Revision: baseline.Revision, Input: validSetup(t)}
	next.Profile = settings.Profile{AcceptedResolutions: []string{"2160p", "1080p"}, PreferredResolution: "2160p", AudioPreferences: []string{"original", "en", "pt"}}
	response := setupRequest(t, handler, "PUT", "/api/v1/settings", encoded(t, next), false)
	if response.Code != 200 {
		t.Fatalf("PUT settings: %d %s", response.Code, response.Body.String())
	}
	saved := readSettings(t, handler)
	if saved.Revision != 2 || saved.Profile.PreferredResolution != "2160p" || !reflect.DeepEqual(saved.Profile.AcceptedResolutions, []string{"1080p", "2160p"}) || !reflect.DeepEqual(saved.Profile.AudioPreferences, next.Profile.AudioPreferences) {
		t.Fatalf("update not fully saved: %+v", saved)
	}
	if response := setupRequest(t, handler, "PUT", "/api/v1/settings", encoded(t, next), false); response.Code != 409 || !strings.Contains(response.Body.String(), "revision-conflict") {
		t.Fatalf("stale update: %d %s", response.Code, response.Body.String())
	}
	data := store.DataDirectory()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.Open(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	after := readSettings(t, New(reopened, fstest.MapFS{}, "127.0.0.1:8080"))
	if !reflect.DeepEqual(after, saved) {
		t.Fatalf("settings changed across restart: %+v != %+v", after, saved)
	}
	contents, err := os.ReadFile(sentinel)
	if err != nil || string(contents) != "existing media" {
		t.Fatalf("old library file was changed: %v", err)
	}
	entries, err := os.ReadDir(saved.Directories.MovieDirectory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("editing roots moved files or left probes: %v %v", entries, err)
	}
}

type coordinatedUpdateStore struct {
	*storage.Store
	arrived chan struct{}
	release chan struct{}
}

func (s *coordinatedUpdateStore) UpdateSettings(ctx context.Context, revision int, input settings.Input) (settings.Settings, error) {
	s.arrived <- struct{}{}
	select {
	case <-s.release:
		return s.Store.UpdateSettings(ctx, revision, input)
	case <-ctx.Done():
		return settings.Settings{}, ctx.Err()
	}
}

func TestSimultaneousSettingsUpdatesHaveOneWinner(t *testing.T) {
	store, _, baseline := configuredAPI(t)
	coordinated := &coordinatedUpdateStore{Store: store, arrived: make(chan struct{}, 2), release: make(chan struct{})}
	handler := New(coordinated, fstest.MapFS{}, "127.0.0.1:8080")
	released := false
	defer func() {
		if !released {
			close(coordinated.release)
		}
	}()
	inputs := []settings.UpdateInput{{Revision: baseline.Revision, Input: validSetup(t)}, {Revision: baseline.Revision, Input: validSetup(t)}}
	inputs[0].Profile.AudioPreferences = []string{"en", "original"}
	inputs[1].Profile.AudioPreferences = []string{"ja", "pt"}
	bodies := [][]byte{encoded(t, inputs[0]), encoded(t, inputs[1])}
	responses := make([]*httptest.ResponseRecorder, 2)
	start := make(chan struct{})
	var group sync.WaitGroup
	for index := range 2 {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			responses[index] = setupRequest(t, handler, "PUT", "/api/v1/settings", bodies[index], false)
		}(index)
	}
	close(start)
	// Both requests have read the same revision before either write begins.
	// The transaction-level comparison must still prevent a second success.
	for range 2 {
		select {
		case <-coordinated.arrived:
		case <-time.After(3 * time.Second):
			t.Fatal("updates did not reach the controlled write boundary")
		}
	}
	close(coordinated.release)
	released = true
	group.Wait()
	winner := -1
	for index, response := range responses {
		if response.Code == 200 {
			if winner != -1 {
				t.Fatal("both updates committed")
			}
			winner = index
		} else if response.Code != 409 {
			t.Fatalf("unexpected competing status: %d %s", response.Code, response.Body.String())
		}
	}
	if winner == -1 {
		t.Fatal("neither update committed")
	}
	saved := readSettings(t, handler)
	if saved.Revision != baseline.Revision+1 || !reflect.DeepEqual(saved.Profile.AudioPreferences, inputs[winner].Profile.AudioPreferences) {
		t.Fatalf("mixed or double-applied updates: %+v", saved)
	}
	expectedMovies, _ := filepath.EvalSymlinks(inputs[winner].Directories.MovieDirectory)
	if saved.Directories.MovieDirectory != expectedMovies {
		t.Fatal("profile and directory belong to different updates")
	}
}

func TestSettingsUpdateValidationLeavesAllValuesUnchanged(t *testing.T) {
	store, handler, baseline := configuredAPI(t)
	base := settings.UpdateInput{Revision: baseline.Revision, Input: settings.Input{Directories: baseline.Directories, Profile: baseline.Profile}}
	invalid := []settings.UpdateInput{}
	revision := base
	revision.Revision = 0
	invalid = append(invalid, revision)
	empty := base
	empty.Profile = settings.Profile{AcceptedResolutions: []string{}, PreferredResolution: "1080p", AudioPreferences: []string{"en"}}
	invalid = append(invalid, empty)
	audio := base
	audio.Profile.AudioPreferences = []string{"original", "original"}
	invalid = append(invalid, audio)
	preferred := base
	preferred.Profile.PreferredResolution = "2160p"
	invalid = append(invalid, preferred)
	relative := base
	relative.Directories.MovieDirectory = "relative"
	invalid = append(invalid, relative)
	overlapping := base
	overlapping.Directories.MovieDirectory = base.Directories.DownloadDirectory
	invalid = append(invalid, overlapping)
	missing := base
	missing.Directories.MovieDirectory = filepath.Join(t.TempDir(), "missing")
	invalid = append(invalid, missing)
	appData := base
	appData.Directories.MovieDirectory = store.DataDirectory()
	invalid = append(invalid, appData)
	for _, input := range invalid {
		response := setupRequest(t, handler, "PUT", "/api/v1/settings", encoded(t, input), false)
		if response.Code != 422 {
			t.Errorf("invalid update: %d %s", response.Code, response.Body.String())
		}
		if after := readSettings(t, handler); !reflect.DeepEqual(after, baseline) {
			t.Fatal("invalid update changed persisted settings")
		}
	}
}

func TestSettingsUpdateStrictJSONAndAccessGuards(t *testing.T) {
	_, handler, baseline := configuredAPI(t)
	for _, body := range []string{`{"revision":1,"revision":1}`, `{"Revision":1}`, `{"revision":"1"}`, `{"revision":1.5}`, `{"revision":1,"workers":2}`, `{"revision":1,"profile":{"audioPreferences":[],"audioPreferences":[]}}`, `{} {}`} {
		response := setupRequest(t, handler, "PUT", "/api/v1/settings", []byte(body), false)
		if response.Code != 400 {
			t.Errorf("malformed update %q: %d %s", body, response.Code, response.Body.String())
		}
	}
	for _, chunked := range []bool{false, true} {
		response := setupRequest(t, handler, "PUT", "/api/v1/settings", []byte(`{"revision":1,"profile":"`+strings.Repeat("x", 65536)+`"}`), chunked)
		if response.Code != 413 {
			t.Errorf("large update chunked=%v: %d", chunked, response.Code)
		}
	}
	for _, test := range []struct {
		header, origin string
		status         int
	}{{"", "", 403}, {"1", "http://evil.example", 403}} {
		request := httptest.NewRequest("PUT", "http://localhost:8080/api/v1/settings", strings.NewReader("{}"))
		request.Header.Set("Content-Type", "application/json")
		if test.header != "" {
			request.Header.Set("X-Piratarr-Request", test.header)
		}
		if test.origin != "" {
			request.Header.Set("Origin", test.origin)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Errorf("access guard: %d", response.Code)
		}
	}
	if after := readSettings(t, handler); !reflect.DeepEqual(after, baseline) {
		t.Fatal("rejected request changed settings")
	}
}

func TestSettingsUpdateBeforeSetupRequiresSetup(t *testing.T) {
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := New(store, fstest.MapFS{}, "127.0.0.1:8080")
	response := setupRequest(t, handler, "PUT", "/api/v1/settings", encoded(t, settings.UpdateInput{Revision: 1, Input: validSetup(t)}), false)
	if response.Code != 409 || !strings.Contains(response.Body.String(), "setup-required") {
		t.Fatalf("pre-setup update: %d %s", response.Code, response.Body.String())
	}
}
