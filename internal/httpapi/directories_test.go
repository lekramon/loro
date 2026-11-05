package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lekramon/piratarr/internal/settings"
	"github.com/lekramon/piratarr/internal/storage"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestDirectoryBrowsingAndExplicitDefaults(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "private")
	if err := os.Mkdir(data, 0700); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := New(store, fstest.MapFS{}, "127.0.0.1:8080")
	h.defaultRoot = filepath.Join(root, "defaults")
	if err = os.WriteFile(filepath.Join(root, "not-a-folder"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	response := setupRequest(t, h, "GET", "/api/v1/directories?path="+url.QueryEscape(root), nil, false)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var listing struct {
		Directories []string `json:"directories"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	if len(listing.Directories) != 0 {
		t.Fatalf("listed private data or files: %v", listing.Directories)
	}
	if _, err = os.Stat(h.defaultRoot); !os.IsNotExist(err) {
		t.Fatal("GET created defaults")
	}
	response = setupRequest(t, h, "GET", "/api/v1/directories?path="+url.QueryEscape(data), nil, false)
	if response.Code != 403 {
		t.Fatal(response.Code)
	}
	response = setupRequest(t, h, "GET", "/api/v1/directories/defaults", nil, false)
	if response.Code != 405 {
		t.Fatal(response.Code)
	}
	for range 2 {
		response = setupRequest(t, h, "POST", "/api/v1/directories/defaults", []byte("{}"), false)
		if response.Code != 200 {
			t.Fatal(response.Body.String())
		}
	}
	var dirs settings.Directories
	if err = json.Unmarshal(response.Body.Bytes(), &dirs); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{dirs.DownloadDirectory, dirs.MovieDirectory} {
		info, e := os.Stat(p)
		if e != nil || !info.IsDir() {
			t.Fatal("default missing", e)
		}
	}
	completed, _ := store.SetupCompleted(context.Background())
	if completed {
		t.Fatal("folder creation completed setup")
	}
	h.defaultRoot = filepath.Join(data, "should-not-exist")
	response = setupRequest(t, h, "POST", "/api/v1/directories/defaults", []byte("{ }"), false)
	if response.Code != 422 {
		t.Fatal(response.Code)
	}
	if _, err = os.Stat(h.defaultRoot); !os.IsNotExist(err) {
		t.Fatal("created defaults inside application data")
	}
	response = setupRequest(t, h, "POST", "/api/v1/directories/defaults", []byte(`{"unexpected":true}`), false)
	if response.Code != 400 {
		t.Fatal(response.Code)
	}
}

func TestQualityPreferencesPersistAndLegacyUpdatesPreserveExclusions(t *testing.T) {
	data := t.TempDir()
	store, err := storage.Open(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	h := New(store, fstest.MapFS{}, "127.0.0.1:8080")
	input := validSetup(t)
	input.Profile.AcceptedFormats = []string{"web-dl"}
	input.Profile.AcceptedContainers = []string{"mkv"}
	response := setupRequest(t, h, "POST", "/api/v1/setup", encoded(t, input), false)
	if response.Code != 201 {
		t.Fatal(response.Body.String())
	}
	store.Close()
	store, err = storage.Open(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h = New(store, fstest.MapFS{}, "127.0.0.1:8080")
	saved, err := store.GetSettings(context.Background())
	if err != nil || len(saved.Profile.AcceptedFormats) != 1 || saved.Profile.AcceptedFormats[0] != "web-dl" || saved.Profile.AcceptedContainers[0] != "mkv" {
		t.Fatal(saved, err)
	}
	input.Profile.AcceptedFormats = nil
	input.Profile.AcceptedContainers = nil
	response = setupRequest(t, h, "PUT", "/api/v1/settings", encoded(t, settings.UpdateInput{Revision: 1, Input: input}), false)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	saved, _ = store.GetSettings(context.Background())
	if saved.Revision != 2 || len(saved.Profile.AcceptedFormats) != 1 || saved.Profile.AcceptedFormats[0] != "web-dl" {
		t.Fatal("legacy update reset exclusions", saved)
	}
	for _, values := range [][]string{{}, {"unknown"}, {"remux", "remux"}} {
		input.Profile.AcceptedFormats = values
		response = setupRequest(t, h, "PUT", "/api/v1/settings", encoded(t, settings.UpdateInput{Revision: 2, Input: input}), false)
		if response.Code != 422 {
			t.Fatal(response.Code, response.Body.String())
		}
	}
	saved, _ = store.GetSettings(context.Background())
	if saved.Revision != 2 {
		t.Fatal("invalid profile changed revision")
	}
}
