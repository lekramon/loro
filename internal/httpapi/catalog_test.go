package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/storage"
	"github.com/lekramon/piratarr/internal/tmdb"
)

type catalogFixture struct {
	store                                *storage.Store
	service                              *catalog.Service
	provider                             *httptest.Server
	client                               *tmdb.Client
	handler                              *Handler
	data, tokenFile                      string
	mode                                 atomic.Int32
	searchCalls, detailCalls, imageCalls atomic.Int32
	poster                               atomic.Value
	seenToken                            atomic.Value
	invalidImage                         atomic.Bool
	imageData                            atomic.Value
	image                                []byte
}

func newCatalogFixture(t *testing.T, setup bool) *catalogFixture {
	t.Helper()
	f := &catalogFixture{data: t.TempDir(), tokenFile: filepath.Join(t.TempDir(), "token")}
	f.poster.Store("poster.png")
	f.seenToken.Store("")
	if err := os.WriteFile(f.tokenFile, []byte("dummy-read-token"), 0600); err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	f.image = buffer.Bytes()
	f.imageData.Store(f.image)
	f.provider = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/images/") {
			f.imageCalls.Add(1)
			if r.Header.Get("Authorization") != "" {
				t.Error("API credential sent to image CDN")
			}
			if f.invalidImage.Load() {
				_, _ = w.Write([]byte("<html>not an image</html>"))
				return
			}
			_, _ = w.Write(f.imageData.Load().([]byte))
			return
		}
		f.seenToken.Store(r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") == "" {
			t.Error("missing provider authorization")
		}
		if strings.Contains(r.URL.Path, "/search/") {
			f.searchCalls.Add(1)
		} else {
			f.detailCalls.Add(1)
		}
		switch f.mode.Load() {
		case 401, 403, 404, 429, 500, 503:
			if f.mode.Load() == 429 {
				w.Header().Set("Retry-After", "2")
			}
			w.WriteHeader(int(f.mode.Load()))
			_, _ = w.Write([]byte(`{"private":"dummy-read-token","upstream":"secret-url"}`))
			return
		case 1:
			_, _ = w.Write([]byte(`{"invalid":`))
			return
		case 2:
			<-r.Context().Done()
			return
		case 3:
			_, _ = w.Write(bytes.Repeat([]byte("x"), (1<<20)+1))
			return
		case 4:
			w.Header().Set("Location", f.provider.URL+"/images/redirected.png")
			w.WriteHeader(302)
			return
		}
		id := int64(42)
		if !strings.Contains(r.URL.Path, "/search/") {
			id, _ = strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/3/movie/"), 10, 64)
		}
		title := "The Movie"
		if r.URL.Query().Get("language") == "pt-BR" {
			title = "O Filme"
		}
		movie := map[string]any{"id": id, "title": title, "original_title": "Original Movie", "original_language": "fr", "release_date": "2024-04-12", "runtime": 123, "overview": "A movie from the simulator.", "genres": []map[string]any{{"name": "Drama"}}, "poster_path": "/" + f.poster.Load().(string), "backdrop_path": "/backdrop.png"}
		if r.URL.Query().Get("query") == "unknown" {
			movie["release_date"] = ""
			movie["original_language"] = ""
			movie["runtime"] = 0
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/search/") {
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			_ = json.NewEncoder(w).Encode(map[string]any{"page": page, "total_pages": 3, "total_results": 43, "results": []any{movie}})
		} else {
			_ = json.NewEncoder(w).Encode(movie)
		}
	}))
	t.Cleanup(f.provider.Close)
	f.client = tmdb.New(tmdb.Config{TokenFile: f.tokenFile, LocalTokenFile: filepath.Join(f.data, "credentials", "tmdb-read-token"), APIBase: f.provider.URL + "/3", ImageBase: f.provider.URL + "/images"})
	var err error
	f.store, err = storage.Open(context.Background(), f.data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.store.Close() })
	f.rebind()
	if setup {
		response := setupRequest(t, f.handler, "POST", "/api/v1/setup", encoded(t, validSetup(t)), false)
		if response.Code != 201 {
			t.Fatal(response.Body.String())
		}
	}
	return f
}
func (f *catalogFixture) rebind() {
	f.service = catalog.New(f.store, f.client, f.data)
	f.handler = New(f.store, fstest.MapFS{}, "127.0.0.1:8080", f.service)
}
func (f *catalogFixture) request(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return setupRequest(t, f.handler, method, path, []byte(body), false)
}
func decodeResponse[T any](t *testing.T, response *httptest.ResponseRecorder, status int) T {
	t.Helper()
	if response.Code != status {
		t.Fatalf("expected %d, got %d: %s", status, response.Code, response.Body.String())
	}
	var result T
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

type registrationResponse struct {
	Movie   catalog.Movie `json:"movie"`
	Created bool          `json:"created"`
}

func TestCatalogSearchRegistrationOfflineAndArtwork(t *testing.T) {
	f := newCatalogFixture(t, true)
	response := f.request(t, "GET", "/api/v1/metadata/search?query=movie&language=pt-BR&page=2&year=2024", "")
	search := decodeResponse[catalog.SearchResult](t, response, 200)
	if search.Page != 2 || search.TotalResults != 43 || len(search.Results) != 1 || search.Results[0].OriginalLanguage != "fr" || search.Results[0].Title != "O Filme" || search.Results[0].Year == nil || *search.Results[0].Year != 2024 || !strings.HasPrefix(search.Results[0].PosterURL, catalog.ImagePrefix) {
		t.Fatal(search)
	}
	if !strings.Contains(response.Header().Get("Link"), "/api/v1/about") {
		t.Fatal("missing attribution link")
	}
	f.request(t, "GET", "/api/v1/metadata/search?query=movie&language=pt-BR&page=2&year=2024", "")
	if f.searchCalls.Load() != 1 {
		t.Fatal("fresh cache repeated provider search")
	}
	unknown := decodeResponse[catalog.SearchResult](t, f.request(t, "GET", "/api/v1/metadata/search?query=unknown", ""), 200)
	if unknown.Results[0].Year != nil || unknown.Results[0].RuntimeMinutes != nil || unknown.Results[0].OriginalLanguage != "" {
		t.Fatal("invented unknown metadata")
	}
	saved := decodeResponse[registrationResponse](t, f.request(t, "POST", "/api/v1/catalog/movies", `{"tmdbId":42,"language":"pt-BR"}`), 201)
	if !saved.Created || saved.Movie.ID < 1 || saved.Movie.HasMedia {
		t.Fatal(saved)
	}
	artwork := f.request(t, "GET", saved.Movie.PosterURL, "")
	if artwork.Code != 200 || artwork.Header().Get("Content-Type") != "image/png" || !bytes.Equal(artwork.Body.Bytes(), f.image) {
		t.Fatal("invalid local image")
	}
	f.provider.Close()
	if err := os.Remove(f.tokenFile); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Close(); err != nil {
		t.Fatal(err)
	}
	var err error
	f.store, err = storage.Open(context.Background(), f.data)
	if err != nil {
		t.Fatal(err)
	}
	f.rebind()
	repeated := decodeResponse[registrationResponse](t, f.request(t, "POST", "/api/v1/catalog/movies", `{"tmdbId":42}`), 200)
	if repeated.Created || repeated.Movie.ID != saved.Movie.ID || repeated.Movie.MetadataLanguage != "pt-BR" {
		t.Fatal("repeat mutated original record", repeated)
	}
	listing := decodeResponse[catalog.ListResult](t, f.request(t, "GET", "/api/v1/catalog/movies?limit=1&offset=0&q=Original", ""), 200)
	if listing.Total != 1 || len(listing.Items) != 1 {
		t.Fatal(listing)
	}
	one := decodeResponse[catalog.Movie](t, f.request(t, "GET", fmt.Sprintf("/api/v1/catalog/movies/%d", saved.Movie.ID), ""), 200)
	if one.AddedAt != saved.Movie.AddedAt || one.Title != saved.Movie.Title {
		t.Fatal(one)
	}
	if response = f.request(t, "GET", saved.Movie.PosterURL, ""); response.Code != 200 || !bytes.Equal(response.Body.Bytes(), f.image) {
		t.Fatal("artwork unavailable offline")
	}
	status := decodeResponse[map[string]any](t, f.request(t, "GET", "/api/v1/metadata/status", ""), 200)
	if status["configured"] != false {
		t.Fatal("credential status leaked stale state")
	}
	empty := decodeResponse[catalog.ListResult](t, f.request(t, "GET", "/api/v1/catalog/movies?q=%25", ""), 200)
	if empty.Total != 0 {
		t.Fatal("title filter treated literal percent as wildcard")
	}
}

func TestCatalogConcurrentRegistrationUsesStableIdentity(t *testing.T) {
	f := newCatalogFixture(t, true)
	var group sync.WaitGroup
	responses := make(chan *httptest.ResponseRecorder, 8)
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			responses <- f.request(t, "POST", "/api/v1/catalog/movies", `{"tmdbId":42}`)
		}()
	}
	group.Wait()
	close(responses)
	created := 0
	var id int64
	for response := range responses {
		if response.Code == 201 {
			created++
		}
		if response.Code != 200 && response.Code != 201 {
			t.Fatal(response.Body.String())
		}
		var saved registrationResponse
		_ = json.Unmarshal(response.Body.Bytes(), &saved)
		if id == 0 {
			id = saved.Movie.ID
		}
		if saved.Movie.ID != id {
			t.Fatal("duplicate title identity")
		}
	}
	if created != 1 || f.detailCalls.Load() != 1 {
		t.Fatalf("created=%d detail requests=%d", created, f.detailCalls.Load())
	}
	listing := decodeResponse[catalog.ListResult](t, f.request(t, "GET", "/api/v1/catalog/movies", ""), 200)
	if listing.Total != 1 {
		t.Fatal(listing)
	}
}

func TestMetadataErrorsAndRetryBounds(t *testing.T) {
	for _, tc := range []struct {
		mode             int32
		status, attempts int
		code             string
	}{{401, 503, 1, "metadata-authentication-failed"}, {403, 503, 1, "metadata-authentication-failed"}, {404, 404, 1, "metadata-not-found"}, {429, 429, 1, "metadata-rate-limited"}, {503, 503, 4, "metadata-unavailable"}, {1, 502, 1, "metadata-invalid-response"}, {3, 502, 1, "metadata-invalid-response"}, {4, 502, 1, "metadata-invalid-response"}} {
		t.Run(tc.code+strconv.Itoa(int(tc.mode)), func(t *testing.T) {
			f := newCatalogFixture(t, true)
			f.mode.Store(tc.mode)
			response := f.request(t, "GET", "/api/v1/metadata/search?query=error", "")
			failure := decodeResponse[apiError](t, response, tc.status)
			if failure.Code != tc.code || int(f.searchCalls.Load()) != tc.attempts {
				t.Fatal(failure, f.searchCalls.Load())
			}
			if strings.Contains(response.Body.String(), "dummy-read-token") || strings.Contains(response.Body.String(), "secret-url") {
				t.Fatal("provider error leaked private data")
			}
			if tc.mode == 429 {
				if response.Header().Get("Retry-After") != "2" {
					t.Fatal("missing retry interval")
				}
				f.request(t, "GET", "/api/v1/metadata/search?query=another", "")
				if f.searchCalls.Load() != 1 {
					t.Fatal("rate-limit cooldown ignored")
				}
			}
			if tc.mode == 4 && f.imageCalls.Load() != 0 {
				t.Fatal("followed provider redirect")
			}
		})
	}
	f := newCatalogFixture(t, true)
	f.mode.Store(2)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	request := httptest.NewRequest("GET", "http://localhost:8080/api/v1/metadata/search?query=timeout", nil).WithContext(ctx)
	response := httptest.NewRecorder()
	start := time.Now()
	f.handler.ServeHTTP(response, request)
	if response.Code != 504 || time.Since(start) > time.Second || f.searchCalls.Load() != 1 {
		t.Fatal("provider timeout was not bounded", response.Code)
	}
}

func TestMetadataStaleCacheCredentialRotationAndValidation(t *testing.T) {
	f := newCatalogFixture(t, false)
	listing := decodeResponse[catalog.ListResult](t, f.request(t, "GET", "/api/v1/catalog/movies", ""), 200)
	if listing.Total != 0 || listing.Items == nil {
		t.Fatal(listing)
	}
	if f.request(t, "POST", "/api/v1/catalog/movies", `{"tmdbId":42}`).Code != 409 {
		t.Fatal("registration ignored setup")
	}
	for _, body := range []string{`{"tmdbId":42,"tmdbId":43}`, `{"TMDBID":42}`, `{"tmdbId":null}`, `{"tmdbId":42} {}`, `{"tmdbId":42,"token":"x"}`, `[]`, `{"tmdbId":"42"}`, `{"tmdbId":42,"language":null}`} {
		if response := f.request(t, "POST", "/api/v1/catalog/movies", body); response.Code != 400 {
			t.Fatal(body, response.Code, response.Body.String())
		}
	}
	for _, body := range []string{`{"tmdbId":0}`, `{"tmdbId":42,"language":"xx"}`} {
		if response := f.request(t, "POST", "/api/v1/catalog/movies", body); response.Code != 422 {
			t.Fatal(body, response.Code)
		}
	}
	for _, path := range []string{"/api/v1/metadata/search?query=", "/api/v1/metadata/search?query=x&page=0", "/api/v1/metadata/search?query=x&language=xx", "/api/v1/metadata/search?query=x&query=y", "/api/v1/catalog/movies?limit=0", "/api/v1/catalog/movies?offset=-1"} {
		if response := f.request(t, "GET", path, ""); response.Code != 422 {
			t.Fatal(path, response.Code)
		}
	}
	if f.searchCalls.Load() != 0 || f.detailCalls.Load() != 0 {
		t.Fatal("invalid requests reached provider")
	}
	if err := os.Remove(f.tokenFile); err != nil {
		t.Fatal(err)
	}
	decodeResponse[apiError](t, f.request(t, "GET", "/api/v1/metadata/search?query=missing", ""), 503)
	if err := os.WriteFile(f.tokenFile, []byte("rotated-dummy-token"), 0600); err != nil {
		t.Fatal(err)
	}
	response := f.request(t, "GET", "/api/v1/metadata/search?query=cached", "")
	fresh := decodeResponse[catalog.SearchResult](t, response, 200)
	if fresh.Cache.Hit || f.seenToken.Load().(string) != "Bearer rotated-dummy-token" {
		t.Fatal("credential rotation not loaded")
	}
	base := time.Now()
	f.service.Now = func() time.Time { return base.Add(25 * time.Hour) }
	f.mode.Store(503)
	stale := decodeResponse[catalog.SearchResult](t, f.request(t, "GET", "/api/v1/metadata/search?query=cached", ""), 200)
	if !stale.Cache.Hit || !stale.Cache.Stale || stale.Warning != "metadata-unavailable" || len(stale.Results) != 1 {
		t.Fatal(stale)
	}
}

func TestArtworkValidationBoundariesAndEviction(t *testing.T) {
	f := newCatalogFixture(t, true)
	f.request(t, "GET", "/api/v1/metadata/search?query=art1", "")
	for _, path := range []string{catalog.ImagePrefix + "original/poster.png", catalog.ImagePrefix + "w342/unobserved.png", catalog.ImagePrefix + "w342/../secret", catalog.ImagePrefix + "w342/evil.svg"} {
		if response := f.request(t, "GET", path, ""); response.Code != 404 {
			t.Fatal(path, response.Code)
		}
	}
	if f.imageCalls.Load() != 0 {
		t.Fatal("unsafe artwork reached CDN")
	}
	f.invalidImage.Store(true)
	response := f.request(t, "GET", catalog.ImagePrefix+"w342/poster.png", "")
	if response.Code != 502 {
		t.Fatal("accepted invalid image", response.Code)
	}
	files, _ := os.ReadDir(filepath.Join(f.data, "cache", "tmdb"))
	if len(files) != 0 {
		t.Fatal("invalid artwork was written")
	}
	f.invalidImage.Store(false)
	f.service.ImageCacheBytes = int64(len(f.image) + 1)
	if f.request(t, "GET", catalog.ImagePrefix+"w342/poster.png", "").Code != 200 {
		t.Fatal("image unavailable")
	}
	f.poster.Store("second.png")
	f.request(t, "GET", "/api/v1/metadata/search?query=art2", "")
	if f.request(t, "GET", catalog.ImagePrefix+"w342/second.png", "").Code != 200 {
		t.Fatal("second image unavailable")
	}
	before := f.imageCalls.Load()
	f.request(t, "GET", catalog.ImagePrefix+"w342/poster.png", "")
	if f.imageCalls.Load() != before+1 {
		t.Fatal("artwork cache limit did not evict oldest file")
	}
}

func TestArtworkRejectsRedirectedCacheDirectory(t *testing.T) {
	f := newCatalogFixture(t, true)
	f.request(t, "GET", "/api/v1/metadata/search?query=art", "")
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(f.data, "cache")); err != nil {
		t.Skip("directory symlink unavailable:", err)
	}
	if response := f.request(t, "GET", catalog.ImagePrefix+"w342/poster.png", ""); response.Code != 503 {
		t.Fatal(response.Code)
	}
	files, err := os.ReadDir(outside)
	if err != nil || len(files) != 0 {
		t.Fatal("image escaped private data directory")
	}
}

func TestArtworkRejectsOversizedAndExcessiveDimensions(t *testing.T) {
	f := newCatalogFixture(t, true)
	f.request(t, "GET", "/api/v1/metadata/search?query=art", "")
	f.imageData.Store(append(append([]byte{}, f.image...), make([]byte, 8<<20)...))
	if response := f.request(t, "GET", catalog.ImagePrefix+"w342/poster.png", ""); response.Code != 502 {
		t.Fatal("accepted oversized image", response.Code)
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 5000, 1))); err != nil {
		t.Fatal(err)
	}
	f.imageData.Store(buffer.Bytes())
	if response := f.request(t, "GET", catalog.ImagePrefix+"w342/poster.png", ""); response.Code != 502 {
		t.Fatal("accepted excessive image dimensions", response.Code)
	}
	files, _ := os.ReadDir(filepath.Join(f.data, "cache", "tmdb"))
	if len(files) != 0 {
		t.Fatal("invalid artwork persisted")
	}
}

func TestCatalogDatabaseIdentityAcrossServiceInstances(t *testing.T) {
	f := newCatalogFixture(t, true)
	other := New(f.store, fstest.MapFS{}, "127.0.0.1:8080", catalog.New(f.store, f.client, f.data))
	var group sync.WaitGroup
	results := make(chan *httptest.ResponseRecorder, 10)
	for index := range 10 {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			handler := f.handler
			if index%2 == 0 {
				handler = other
			}
			results <- setupRequest(t, handler, "POST", "/api/v1/catalog/movies", []byte(`{"tmdbId":52}`), false)
		}(index)
	}
	group.Wait()
	close(results)
	created := 0
	var id int64
	for response := range results {
		if response.Code == 201 {
			created++
		}
		if response.Code != 200 && response.Code != 201 {
			t.Fatal(response.Code, response.Body.String())
		}
		var result registrationResponse
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if id == 0 {
			id = result.Movie.ID
		}
		if id != result.Movie.ID {
			t.Fatal("separate services created duplicate identity")
		}
	}
	if created != 1 {
		t.Fatal("database did not serialize registration", created)
	}
}

func TestMetadataCredentialConfiguration(t *testing.T) {
	f := newCatalogFixture(t, false)
	if err := os.Remove(f.tokenFile); err != nil {
		t.Fatal(err)
	}
	if f.client.Configured() {
		t.Fatal("unexpected credential")
	}
	token := "dummy-service-token"
	response := f.request(t, "PUT", "/api/v1/metadata/credential", `{"token":"`+token+`"}`)
	if response.Code != 200 || strings.Contains(response.Body.String(), token) {
		t.Fatal(response.Code, response.Body.String())
	}
	if !f.client.Configured() {
		t.Fatal("save did not configure provider")
	}
	for _, body := range []string{`{}`, `{"token":""}`, `{"token":"bad token"}`, `{"token":123}`, `{"token":"x","token":"y"}`, `{"token":"x","extra":true}`, `{"token":"x"} {}`, `[]`} {
		response = f.request(t, "PUT", "/api/v1/metadata/credential", body)
		if response.Code != 400 && response.Code != 422 {
			t.Fatal(response.Code, body)
		}
	}
	response = f.request(t, "GET", "/api/v1/metadata/search?query=first-service-query", "")
	if response.Code != 200 || f.seenToken.Load() != "Bearer "+token {
		t.Fatal(response.Code, "saved token unused")
	}
	// A new provider/handler uses the stored file even with an absent bootstrap file.
	f.client = tmdb.New(tmdb.Config{TokenFile: f.tokenFile, LocalTokenFile: filepath.Join(f.data, "credentials", "tmdb-read-token"), APIBase: f.provider.URL + "/3"})
	f.rebind()
	if !f.client.Configured() {
		t.Fatal("credential lost on restart")
	}
	response = f.request(t, "PUT", "/api/v1/metadata/credential", `{"token":"dummy-replacement"}`)
	if response.Code != 200 {
		t.Fatal(response.Code)
	}
	response = f.request(t, "GET", "/api/v1/metadata/search?query=after-rotation", "")
	if response.Code != 200 || f.seenToken.Load() != "Bearer dummy-replacement" {
		t.Fatal("rotation failed")
	}
	// Existing secret survives a failed atomic replacement.
	path := filepath.Join(f.data, "credentials", "tmdb-read-token")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	response = f.request(t, "PUT", "/api/v1/metadata/credential", `{"token":"dummy-not-saved"}`)
	if response.Code != 503 || strings.Contains(response.Body.String(), "dummy-not-saved") {
		t.Fatal(response.Code)
	}
}

func TestMetadataCredentialMutationGuards(t *testing.T) {
	f := newCatalogFixture(t, false)
	for _, origin := range []string{"", "http://evil.example"} {
		req := httptest.NewRequest("PUT", "http://127.0.0.1:8080/api/v1/metadata/credential", strings.NewReader(`{"token":"dummy-guarded"}`))
		req.Header.Set("Content-Type", "application/json")
		if origin != "" {
			req.Header.Set("Origin", origin)
			req.Header.Set("X-Piratarr-Request", "1")
		}
		response := httptest.NewRecorder()
		f.handler.ServeHTTP(response, req)
		if response.Code != 403 {
			t.Fatal(response.Code)
		}
	}
	if _, err := os.Stat(filepath.Join(f.data, "credentials", "tmdb-read-token")); !os.IsNotExist(err) {
		t.Fatal("rejected mutation wrote credential")
	}
	response := f.request(t, "PUT", "/api/v1/metadata/credential", `{"token":"`+strings.Repeat("x", 65537)+`"}`)
	if response.Code != 413 {
		t.Fatal(response.Code)
	}
}

func TestMetadataConnectionUsesUncachedSavedCredential(t *testing.T) {
	f := newCatalogFixture(t, false)
	if r := f.request(t, "PUT", "/api/v1/metadata/credential", `{"token":"dummy-tested-token"}`); r.Code != 200 {
		t.Fatal(r.Code)
	}
	if r := f.request(t, "GET", "/api/v1/metadata/search?query=Inception", ""); r.Code != 200 {
		t.Fatal(r.Code)
	}
	for attempt := 0; attempt < 2; attempt++ {
		before := f.searchCalls.Load()
		r := f.request(t, "GET", "/api/v1/metadata/test", "")
		if r.Code != 200 || !strings.Contains(r.Body.String(), `"connected":true`) || strings.Contains(r.Body.String(), "dummy-tested-token") {
			t.Fatal(r.Code, r.Body.String())
		}
		if f.searchCalls.Load() != before+1 || f.seenToken.Load() != "Bearer dummy-tested-token" {
			t.Fatal("connection test used cache or wrong credential")
		}
	}
}
func TestMetadataConnectionFailures(t *testing.T) {
	for _, scenario := range []struct {
		mode   int32
		status int
		code   string
	}{{401, 503, "metadata-authentication-failed"}, {429, 429, "metadata-rate-limited"}, {503, 503, "metadata-unavailable"}, {1, 502, "metadata-invalid-response"}, {2, 504, "metadata-timeout"}} {
		t.Run(scenario.code, func(t *testing.T) {
			f := newCatalogFixture(t, false)
			f.mode.Store(scenario.mode)
			r := f.request(t, "GET", "/api/v1/metadata/test", "")
			if r.Code != scenario.status || !strings.Contains(r.Body.String(), scenario.code) || strings.Contains(r.Body.String(), "dummy-read-token") {
				t.Fatal(r.Code, r.Body.String())
			}
			if scenario.status == 429 && r.Header().Get("Retry-After") == "" {
				t.Fatal("missing retry-after")
			}
		})
	}
	f := newCatalogFixture(t, false)
	os.Remove(f.tokenFile)
	r := f.request(t, "GET", "/api/v1/metadata/test", "")
	if r.Code != 503 || !strings.Contains(r.Body.String(), "metadata-unconfigured") {
		t.Fatal(r.Code, r.Body.String())
	}
}
