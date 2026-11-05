package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lekramon/piratarr/internal/sources"
	"github.com/lekramon/piratarr/internal/storage"
)

const sourceKey = "dummy-source-key"
const sourceCaps = `<?xml version="1.0"?><caps><server title="ignored dummy-source-key" url="http://do-not-fetch.invalid"/><limits max="100" default="50"/><searching><search available="yes" supportedParams="q,year"/><movie-search available="yes" supportedParams="q,imdbid,tmdbid"/></searching><categories><category id="2000" name="Movies dummy-source-key"><subcat id="2040" name="HD"/></category></categories></caps>`

type sourceFixture struct {
	*catalogFixture
	server                 *httptest.Server
	calls, mode, redirects atomic.Int32
	key                    atomic.Value
}

func newSourceFixture(t *testing.T) *sourceFixture {
	f := &sourceFixture{catalogFixture: newCatalogFixture(t, true)}
	f.key.Store("")
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect-target" {
			f.redirects.Add(1)
			fmt.Fprint(w, sourceCaps)
			return
		}
		f.calls.Add(1)
		f.key.Store(r.URL.Query().Get("apikey"))
		if r.URL.Query().Get("t") != "caps" || r.URL.Path != "/custom/api" {
			t.Error("unexpected endpoint/function")
		}
		switch f.mode.Load() {
		case 401, 403, 429, 500, 503:
			if f.mode.Load() == 429 {
				w.Header().Set("Retry-After", "2")
			}
			w.WriteHeader(int(f.mode.Load()))
			fmt.Fprint(w, sourceKey)
			return
		case 1:
			fmt.Fprint(w, `<error code="100" description="private dummy-source-key"/>`)
			return
		case 2:
			<-r.Context().Done()
			return
		case 3:
			fmt.Fprint(w, strings.Repeat("x", (2<<20)+1))
			return
		case 4:
			w.Header().Set("Location", f.server.URL+"/redirect-target")
			w.WriteHeader(302)
			return
		case 5:
			fmt.Fprint(w, `<!DOCTYPE caps SYSTEM "http://do-not-fetch.invalid"><caps/>`)
			return
		case 6:
			fmt.Fprint(w, `<html>private dummy-source-key</html>`)
			return
		}
		fmt.Fprint(w, sourceCaps)
	}))
	t.Cleanup(f.server.Close)
	return f
}
func (f *sourceFixture) save(t *testing.T, revision int, key string) sources.Public {
	body := fmt.Sprintf(`{"revision":%d,"name":"Test source","url":%q,"apiKey":%q}`, revision, f.server.URL+"/custom/api", key)
	response := f.request(t, "PUT", "/api/v1/source", body)
	if strings.Contains(response.Body.String(), sourceKey) {
		t.Fatal("credential exposed")
	}
	return decodeResponse[sources.Public](t, response, 200)
}
func TestSourceSaveCapabilitiesRotationAndRestart(t *testing.T) {
	f := newSourceFixture(t)
	initial := decodeResponse[sources.Public](t, f.request(t, "GET", "/api/v1/source", ""), 200)
	if initial.Configured || initial.Revision != 0 || initial.Capabilities != nil {
		t.Fatal(initial)
	}
	first := f.save(t, 0, sourceKey)
	if !first.HasAPIKey || first.Revision != 1 || f.calls.Load() != 0 {
		t.Fatal(first)
	}
	result := f.request(t, "GET", "/api/v1/source/test", "")
	tested := decodeResponse[sources.TestResult](t, result, 200)
	if !tested.Connected || !tested.Capabilities.MovieSearch.Available || len(tested.Capabilities.Categories) != 2 || strings.Contains(result.Body.String(), sourceKey) {
		t.Fatal(result.Code, result.Body.String())
	}
	if f.key.Load() != sourceKey {
		t.Fatal("saved credential not sent")
	}
	f.request(t, "GET", "/api/v1/source/test", "")
	if f.calls.Load() != 2 {
		t.Fatal("test used cached capabilities")
	}
	f.mode.Store(401)
	failed := f.request(t, "GET", "/api/v1/source/test", "")
	if failed.Code != 503 {
		t.Fatal(failed.Code)
	}
	prior := decodeResponse[sources.Public](t, f.request(t, "GET", "/api/v1/source", ""), 200)
	if prior.CheckedAt == nil || prior.Capabilities == nil {
		t.Fatal("failed test erased last successful discovery")
	}
	f.mode.Store(0)
	f.store.Close()
	var err error
	f.store, err = storage.Open(context.Background(), f.data)
	if err != nil {
		t.Fatal(err)
	}
	f.rebind()
	reloaded := decodeResponse[sources.Public](t, f.request(t, "GET", "/api/v1/source", ""), 200)
	if reloaded.Revision != 1 || !reloaded.HasAPIKey || reloaded.CheckedAt == nil {
		t.Fatal(reloaded)
	}
	renamed := f.save(t, 1, "")
	if !renamed.HasAPIKey || renamed.Capabilities != nil {
		t.Fatal(renamed)
	}
	if r := f.request(t, "GET", "/api/v1/source/test", ""); r.Code != 200 || f.key.Load() != sourceKey {
		t.Fatal(r.Code)
	}
	cleared := fmt.Sprintf(`{"revision":2,"name":"Public source","url":%q,"clearApiKey":true}`, f.server.URL+"/custom/api")
	saved := decodeResponse[sources.Public](t, f.request(t, "PUT", "/api/v1/source", cleared), 200)
	if saved.HasAPIKey {
		t.Fatal(saved)
	}
	if r := f.request(t, "GET", "/api/v1/source/test", ""); r.Code != 200 || f.key.Load() != "" {
		t.Fatal(r.Code)
	}
}
func TestSourceFailureClassificationAndRetryBounds(t *testing.T) {
	for _, scenario := range []struct {
		mode          int32
		status, calls int
		code          string
	}{{401, 503, 1, "source-authentication-failed"}, {1, 503, 1, "source-authentication-failed"}, {429, 429, 1, "source-rate-limited"}, {503, 503, 4, "source-unavailable"}, {3, 502, 1, "source-invalid-response"}, {4, 502, 1, "source-invalid-response"}, {5, 502, 1, "source-invalid-response"}, {6, 502, 1, "source-invalid-response"}} {
		t.Run(fmt.Sprint(scenario.mode), func(t *testing.T) {
			f := newSourceFixture(t)
			f.save(t, 0, sourceKey)
			f.mode.Store(scenario.mode)
			r := f.request(t, "GET", "/api/v1/source/test", "")
			if r.Code != scenario.status || !strings.Contains(r.Body.String(), scenario.code) || strings.Contains(r.Body.String(), sourceKey) || strings.Contains(r.Body.String(), f.server.URL) {
				t.Fatal(r.Code, r.Body.String())
			}
			if health := f.request(t, "GET", "/api/v1/health/ready", ""); health.Code != 200 {
				t.Fatal("source outage affected readiness", health.Code)
			}
			if int(f.calls.Load()) != scenario.calls || f.redirects.Load() != 0 {
				t.Fatal("request bound or redirect failure", f.calls.Load())
			}
			if scenario.mode == 429 {
				if r.Header().Get("Retry-After") == "" {
					t.Fatal("missing cooldown")
				}
				f.request(t, "GET", "/api/v1/source/test", "")
				if f.calls.Load() != 1 {
					t.Fatal("cooldown ignored")
				}
				f.save(t, 1, "")
				f.mode.Store(0)
				if r = f.request(t, "GET", "/api/v1/source/test", ""); r.Code != 200 {
					t.Fatal("old cooldown affected new revision", r.Code)
				}
			}
		})
	}
	f := newSourceFixture(t)
	f.save(t, 0, sourceKey)
	f.mode.Store(2)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	request := httptest.NewRequest("GET", "http://localhost:8080/api/v1/source/test", nil).WithContext(ctx)
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, request)
	if response.Code != 504 {
		t.Fatal(response.Code, response.Body.String())
	}
}
func TestSourceConfigurationValidationAndSecretDestination(t *testing.T) {
	f := newSourceFixture(t)
	f.save(t, 0, sourceKey)
	for _, body := range []string{`null`, `{"revision":1,"revision":1}`, `{"revision":null}`, `{"revision":1,"extra":true}`} {
		r := f.request(t, "PUT", "/api/v1/source", body)
		if r.Code != 400 {
			t.Fatal(body, r.Code)
		}
	}
	for _, endpoint := range []string{"file:///secret", "https://user:pass@host/api", "http://host/api?apikey=secret", "http://host/api#secret", "http://host:0/api", "http://host:99999/api", "http://host:/api"} {
		body := fmt.Sprintf(`{"revision":1,"name":"Source","url":%q,"apiKey":"new"}`, endpoint)
		r := f.request(t, "PUT", "/api/v1/source", body)
		if r.Code != 422 || strings.Contains(r.Body.String(), endpoint) {
			t.Fatal(endpoint, r.Code)
		}
	}
	changedEndpoint := fmt.Sprintf(`{"revision":1,"name":"Source","url":%q}`, f.server.URL+"/other/api")
	if r := f.request(t, "PUT", "/api/v1/source", changedEndpoint); r.Code != 422 {
		t.Fatal("old key reused for changed endpoint", r.Code)
	}
	stale := fmt.Sprintf(`{"revision":0,"name":"Source","url":%q}`, f.server.URL+"/custom/api")
	if r := f.request(t, "PUT", "/api/v1/source", stale); r.Code != 409 {
		t.Fatal(r.Code)
	}
	if r := f.request(t, "PUT", "/api/v1/source", strings.Repeat("x", 65537)); r.Code != 413 {
		t.Fatal(r.Code)
	}
	request := httptest.NewRequest("PUT", "http://localhost:8080/api/v1/source", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	r := httptest.NewRecorder()
	f.handler.ServeHTTP(r, request)
	if r.Code != 403 {
		t.Fatal(r.Code)
	}
}
func TestSourceTestConcurrencyAndConfigChange(t *testing.T) {
	f := newCatalogFixture(t, true)
	started, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-release; fmt.Fprint(w, sourceCaps) }))
	defer server.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	body := fmt.Sprintf(`{"revision":0,"name":"Source","url":%q,"apiKey":%q}`, server.URL, sourceKey)
	decodeResponse[sources.Public](t, f.request(t, "PUT", "/api/v1/source", body), 200)
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- f.request(t, "GET", "/api/v1/source/test", "") }()
	<-started
	if r := f.request(t, "GET", "/api/v1/source/test", ""); r.Code != 409 {
		t.Fatal(r.Code)
	}
	body = fmt.Sprintf(`{"revision":1,"name":"Changed","url":%q}`, server.URL)
	decodeResponse[sources.Public](t, f.request(t, "PUT", "/api/v1/source", body), 200)
	close(release)
	if r := <-done; r.Code != 409 || !strings.Contains(r.Body.String(), "source-changed") {
		t.Fatal(r.Code, r.Body.String())
	}
	saved := decodeResponse[sources.Public](t, f.request(t, "GET", "/api/v1/source", ""), 200)
	if saved.Revision != 2 || saved.Capabilities != nil {
		t.Fatal(saved)
	}
}

func TestSourceRequiresSetupAndConfiguration(t *testing.T) {
	f := newCatalogFixture(t, false)
	if r := f.request(t, "GET", "/api/v1/source/test", ""); r.Code != 503 || !strings.Contains(r.Body.String(), "source-unconfigured") {
		t.Fatal(r.Code, r.Body.String())
	}
	body := `{"revision":0,"name":"Source","url":"http://localhost:9999/api"}`
	if r := f.request(t, "PUT", "/api/v1/source", body); r.Code != 409 || !strings.Contains(r.Body.String(), "setup-required") {
		t.Fatal(r.Code, r.Body.String())
	}
}

func TestSourceChangedEndpointUsesOnlyExplicitReplacementKey(t *testing.T) {
	f := newSourceFixture(t)
	f.save(t, 0, sourceKey)
	var seen atomic.Value
	seen.Store("")
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.Store(r.URL.Query().Get("apikey"))
		fmt.Fprint(w, `<caps><searching><search available="yes" supportedParams="q"/></searching></caps>`)
	}))
	defer other.Close()
	body := fmt.Sprintf(`{"revision":1,"name":"Replacement","url":%q,"apiKey":"replacement-key"}`, other.URL+"/api")
	decodeResponse[sources.Public](t, f.request(t, "PUT", "/api/v1/source", body), 200)
	if r := f.request(t, "GET", "/api/v1/source/test", ""); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	if seen.Load() != "replacement-key" || f.calls.Load() != 0 {
		t.Fatal("credential or endpoint reused incorrectly")
	}
}
