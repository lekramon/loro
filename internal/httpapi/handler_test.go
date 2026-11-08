package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lekramon/piratarr/internal/storage"
)

func TestFoundationAPIWithRealDatabase(t *testing.T) {
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := New(store, fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>Piratarr</title>")}}, "127.0.0.1:8080")
	request := func(method, path, host, origin string, headers map[string]string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "http://"+host+path, strings.NewReader("{}"))
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		for key, value := range headers {
			r.Header.Set(key, value)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for path, status := range map[string]int{"/api/v1/health/live": 200, "/api/v1/health/ready": 200, "/api/v1/system/operations": 200, "/api/v1/setup": 200, "/api/v1/settings": 409, "/api/v1/missing": 404, "/api/missing": 404, "/api": 404, "/": 200, "/setup": 200, "/missing.js": 404} {
		if w := request("GET", path, "localhost:8080", "", nil); w.Code != status {
			t.Errorf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	setup := request("GET", "/api/v1/setup", "localhost:8080", "", nil)
	var body struct {
		Completed            bool
		SupportedResolutions []string
		Languages            []language
	}
	if err := json.Unmarshal(setup.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Completed || len(body.SupportedResolutions) != 5 || len(body.Languages) != 184 {
		t.Fatalf("incorrect setup options: %+v", body)
	}
	for _, test := range []struct{ host, origin string }{{"evil.example:8080", ""}, {"localhost:9999", ""}, {"localhost:8080", "http://evil.example:8080"}, {"localhost:8080", "null"}, {"localhost:8080", "http://127.0.0.1:8080"}} {
		w := request("GET", "/api/v1/setup", test.host, test.origin, nil)
		if w.Code != 403 {
			t.Errorf("accepted host/origin: %+v", test)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Error("unexpected CORS permission")
		}
	}
	if w := request("GET", "/api/v1/setup", "localhost:8080", "http://localhost:8080", nil); w.Code != 200 {
		t.Fatal("same origin rejected")
	}
	if w := request("OPTIONS", "/api/v1/setup", "localhost:8080", "", nil); w.Code != 403 {
		t.Fatal("preflight accepted")
	}
	if w := request("POST", "/api/v1/setup", "localhost:8080", "", nil); w.Code != 403 {
		t.Fatal("missing mutation header accepted")
	}
	if w := request("POST", "/api/v1/setup", "localhost:8080", "", map[string]string{"X-Piratarr-Request": "1"}); w.Code != 415 {
		t.Fatal("non-JSON accepted")
	}
	if w := request("POST", "/api/v1/setup", "localhost:8080", "", map[string]string{"X-Piratarr-Request": "1", "Content-Type": "application/json"}); w.Code != 422 {
		t.Fatalf("empty setup should require explicit choices: %d", w.Code)
	}
	handler.Stop()
	if w := request("GET", "/api/v1/health/ready", "localhost:8080", "", nil); w.Code != 503 {
		t.Fatal("stopping service reported ready")
	}
	if w := request("GET", "/api/v1/health/live", "localhost:8080", "", nil); w.Code != 200 {
		t.Fatal("stopping service not live")
	}
}

func TestUnavailableStorageReadiness(t *testing.T) {
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := New(store, fstest.MapFS{}, "127.0.0.1:8080")
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/health/ready", "/api/v1/setup"} {
		r := httptest.NewRequest(http.MethodGet, "http://localhost:8080"+path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 503 {
			t.Errorf("unavailable %s: %d", path, w.Code)
		}
	}
}
