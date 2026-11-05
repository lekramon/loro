package httpapi

import (
	"bytes"
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

	"github.com/lekramon/piratarr/internal/settings"
	"github.com/lekramon/piratarr/internal/storage"
)

func setupRequest(t *testing.T, h *Handler, method, path string, body []byte, chunked bool) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, "http://localhost:8080"+path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Piratarr-Request", "1")
	if chunked {
		request.ContentLength = -1
	}
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	return response
}

func validSetup(t *testing.T) settings.Input {
	t.Helper()
	return settings.Input{Directories: settings.Directories{DownloadDirectory: t.TempDir(), MovieDirectory: t.TempDir()}, Profile: settings.Profile{AcceptedResolutions: []string{"1080p", "720p"}, PreferredResolution: "1080p", AudioPreferences: []string{"pt", "original", "en"}}}
}

func encoded(t *testing.T, value any) []byte {
	t.Helper()
	result, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestSetupPersistsAndRejectsRepetition(t *testing.T) {
	data := t.TempDir()
	store, err := storage.Open(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	h := New(store, fstest.MapFS{}, "127.0.0.1:8080")
	input := validSetup(t)
	sentinel := filepath.Join(input.Directories.MovieDirectory, "existing.mkv")
	if err := os.WriteFile(sentinel, []byte("preserve existing media"), 0600); err != nil {
		t.Fatal(err)
	}
	response := setupRequest(t, h, "POST", "/api/v1/setup", encoded(t, input), false)
	if response.Code != 201 {
		t.Fatalf("%d %s", response.Code, response.Body.String())
	}
	var saved settings.Settings
	if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Revision != 1 || saved.UpdatedAt == "" || !reflect.DeepEqual(saved.Profile.AcceptedResolutions, []string{"720p", "1080p"}) || !reflect.DeepEqual(saved.Profile.AudioPreferences, input.Profile.AudioPreferences) {
		t.Fatalf("unexpected saved profile: %+v", saved)
	}
	again := setupRequest(t, h, "POST", "/api/v1/setup", encoded(t, input), false)
	if again.Code != 409 || !strings.Contains(again.Body.String(), "setup-already-completed") {
		t.Fatalf("duplicate setup: %d %s", again.Code, again.Body.String())
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = storage.Open(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h = New(store, fstest.MapFS{}, "127.0.0.1:8080")
	response = setupRequest(t, h, "GET", "/api/v1/settings", nil, false)
	if response.Code != 200 {
		t.Fatalf("read after restart: %d %s", response.Code, response.Body.String())
	}
	var loaded settings.Settings
	_ = json.Unmarshal(response.Body.Bytes(), &loaded)
	if !reflect.DeepEqual(saved, loaded) {
		t.Fatalf("changed across restart: %+v != %+v", saved, loaded)
	}
	for _, root := range []string{input.Directories.DownloadDirectory, input.Directories.MovieDirectory} {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".piratarr-probe-") {
				t.Error("validation left a temporary file")
			}
		}
	}
	contents, _ := os.ReadFile(sentinel)
	if string(contents) != "preserve existing media" {
		t.Fatal("setup altered existing media")
	}
}

func TestSetupConcurrentRequestsCommitOnlyOneProfile(t *testing.T) {
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := New(store, fstest.MapFS{}, "127.0.0.1:8080")
	inputs := []settings.Input{validSetup(t), validSetup(t)}
	inputs[1].Profile.AudioPreferences = []string{"original", "ja"}
	responses := make([]*httptest.ResponseRecorder, 2)
	var group sync.WaitGroup
	start := make(chan struct{})
	bodies := [][]byte{encoded(t, inputs[0]), encoded(t, inputs[1])}
	for index := range 2 {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			responses[index] = setupRequest(t, h, "POST", "/api/v1/setup", bodies[index], false)
		}(index)
	}
	close(start)
	group.Wait()
	winner := -1
	for index, response := range responses {
		if response.Code == 201 {
			if winner != -1 {
				t.Fatal("two setup requests committed")
			}
			winner = index
		} else if response.Code != 409 {
			t.Fatalf("unexpected competing result: %d %s", response.Code, response.Body.String())
		}
	}
	if winner == -1 {
		t.Fatal("neither setup request committed")
	}
	loaded, err := store.GetSettings(context.Background())
	if err != nil || !reflect.DeepEqual(loaded.Profile.AudioPreferences, inputs[winner].Profile.AudioPreferences) {
		t.Fatalf("mixed profile state: %+v %v", loaded, err)
	}
}

func TestSetupRejectsInvalidProfilesWithoutPersisting(t *testing.T) {
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := New(store, fstest.MapFS{}, "127.0.0.1:8080")
	base := validSetup(t)
	for _, profile := range []settings.Profile{
		{},
		{AcceptedResolutions: []string{"1080p", "1080p"}, PreferredResolution: "1080p", AudioPreferences: []string{"en"}},
		{AcceptedResolutions: []string{"8k"}, PreferredResolution: "8k", AudioPreferences: []string{"en"}},
		{AcceptedResolutions: []string{"720p"}, PreferredResolution: "1080p", AudioPreferences: []string{"en"}},
		{AcceptedResolutions: []string{"720p"}, PreferredResolution: "720p", AudioPreferences: []string{"original", "original"}},
		{AcceptedResolutions: []string{"720p"}, PreferredResolution: "720p", AudioPreferences: []string{"zz"}},
		{AcceptedResolutions: []string{"720p"}, PreferredResolution: "720p", AudioPreferences: []string{"EN"}},
	} {
		input := base
		input.Profile = profile
		response := setupRequest(t, h, "POST", "/api/v1/setup", encoded(t, input), false)
		if response.Code != 422 || !strings.Contains(response.Body.String(), "fieldErrors") {
			t.Errorf("invalid profile: %d %s", response.Code, response.Body.String())
		}
	}
	if response := setupRequest(t, h, "GET", "/api/v1/settings", nil, false); response.Code != 409 {
		t.Fatal("invalid settings were persisted")
	}
}

func TestSetupRejectsUnsafeDirectories(t *testing.T) {
	data := t.TempDir()
	store, err := storage.Open(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := New(store, fstest.MapFS{}, "127.0.0.1:8080")
	base := validSetup(t)
	nested := filepath.Join(base.Directories.DownloadDirectory, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	regular := filepath.Join(base.Directories.MovieDirectory, "file")
	if err := os.WriteFile(regular, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, dirs := range []settings.Directories{
		{DownloadDirectory: "relative", MovieDirectory: base.Directories.MovieDirectory},
		{DownloadDirectory: filepath.Join(data, "missing"), MovieDirectory: base.Directories.MovieDirectory},
		{DownloadDirectory: regular, MovieDirectory: base.Directories.MovieDirectory},
		{DownloadDirectory: base.Directories.DownloadDirectory, MovieDirectory: base.Directories.DownloadDirectory},
		{DownloadDirectory: base.Directories.DownloadDirectory, MovieDirectory: nested},
		{DownloadDirectory: data, MovieDirectory: base.Directories.MovieDirectory},
		{DownloadDirectory: filepath.Dir(data), MovieDirectory: base.Directories.MovieDirectory},
	} {
		input := base
		input.Directories = dirs
		response := setupRequest(t, h, "POST", "/api/v1/setup", encoded(t, input), false)
		if response.Code != 422 {
			t.Errorf("unsafe directories: %d %s", response.Code, response.Body.String())
		}
	}
	contents, _ := os.ReadFile(regular)
	if string(contents) != "untouched" {
		t.Fatal("validation modified existing file")
	}
	if completed, _ := store.SetupCompleted(context.Background()); completed {
		t.Fatal("unsafe settings persisted")
	}
}

func TestSetupRejectsMalformedAndOversizedJSON(t *testing.T) {
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := New(store, fstest.MapFS{}, "127.0.0.1:8080")
	for _, body := range []string{"", "null", "[]", "{", "{} {}", "{} trailing", `{"profile":{},"profile":{}}`, `{"profile":{"audioPreferences":[],"audioPreferences":[]}}`, `{"unknown":1}`, `{"Profile":{}}`, `{"profile":{"unknown":true}}`, `{"profile":{"acceptedResolutions":42}}`} {
		response := setupRequest(t, h, "POST", "/api/v1/setup", []byte(body), false)
		if response.Code != 400 {
			t.Errorf("body %q: %d %s", body, response.Code, response.Body.String())
		}
	}
	for _, chunked := range []bool{false, true} {
		response := setupRequest(t, h, "POST", "/api/v1/setup", []byte(`{"profile":"`+strings.Repeat("x", 65536)+`"}`), chunked)
		if response.Code != 413 {
			t.Errorf("oversized chunked=%v: %d", chunked, response.Code)
		}
	}
	if completed, _ := store.SetupCompleted(context.Background()); completed {
		t.Fatal("invalid JSON changed setup state")
	}
}

func TestSetupCanonicalizesDirectoryAliases(t *testing.T) {
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := New(store, fstest.MapFS{}, "127.0.0.1:8080")
	input := validSetup(t)
	target := input.Directories.DownloadDirectory
	alias := filepath.Join(t.TempDir(), "download-alias")
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	input.Directories.DownloadDirectory = alias
	// A second path to the same physical root must not bypass overlap checks.
	originalMovies := input.Directories.MovieDirectory
	input.Directories.MovieDirectory = target
	if response := setupRequest(t, h, "POST", "/api/v1/setup", encoded(t, input), false); response.Code != 422 {
		t.Fatalf("aliased overlapping roots accepted: %d %s", response.Code, response.Body.String())
	}
	input.Directories.MovieDirectory = originalMovies
	response := setupRequest(t, h, "POST", "/api/v1/setup", encoded(t, input), false)
	if response.Code != 201 {
		t.Fatalf("alias: %d %s", response.Code, response.Body.String())
	}
	var result settings.Settings
	_ = json.Unmarshal(response.Body.Bytes(), &result)
	expected, _ := filepath.EvalSymlinks(target)
	if result.Directories.DownloadDirectory != expected {
		t.Errorf("alias not canonicalized: %s", result.Directories.DownloadDirectory)
	}
}
