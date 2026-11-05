package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/settings"
	"github.com/lekramon/piratarr/internal/sources"
	"github.com/lekramon/piratarr/internal/storage"
)

const releaseFeed = `<rss xmlns:torznab="http://torznab.com/schemas/2015/feed"><channel><torznab:response offset="0" total="3"/>
<item><title>Example.2024.1080p.BluRay.Remux.mkv dummy-source-key</title><guid>private-guid-1</guid><pubDate>Thu, 05 Nov 2026 12:00:00 +0000</pubDate><enclosure url="http://do-not-fetch.invalid/file?apikey=dummy-source-key" length="1234"/><torznab:attr name="size" value="1234"/><torznab:attr name="seeders" value="12"/><torznab:attr name="language" value="French"/></item>
<item><title>Example.2160p.WEB-DL.mp4</title><guid>private-guid-2</guid><torznab:attr name="language" value="Spanish"/></item>
<item><title>Example.MULTI.DUAL</title><guid>private-guid-3</guid><torznab:attr name="subs" value="French"/><torznab:attr name="audio" value="AAC"/></item></channel></rss>`

type releaseFixture struct {
	*catalogFixture
	server     *httptest.Server
	feed, caps atomic.Value
	status     atomic.Int32
	calls      atomic.Int32
	block      atomic.Bool
	queries    chan url.Values
	release    chan struct{}
	movie      catalog.Movie
}

func newReleaseFixture(t *testing.T) *releaseFixture {
	f := &releaseFixture{catalogFixture: newCatalogFixture(t, true), queries: make(chan url.Values, 32), release: make(chan struct{})}
	f.feed.Store(releaseFeed)
	f.caps.Store(sourceCaps)
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("t") == "caps" {
			fmt.Fprint(w, f.caps.Load())
			return
		}
		f.calls.Add(1)
		f.queries <- r.URL.Query()
		if f.block.Load() {
			select {
			case <-f.release:
			case <-r.Context().Done():
				return
			}
		}
		if f.status.Load() != 0 {
			if f.status.Load() == 429 {
				w.Header().Set("Retry-After", "2")
			}
			w.WriteHeader(int(f.status.Load()))
			return
		}
		fmt.Fprint(w, f.feed.Load())
	}))
	t.Cleanup(f.server.Close)
	decodeResponse[sources.Public](t, f.request(t, "PUT", "/api/v1/source", fmt.Sprintf(`{"revision":0,"name":"Movie source","url":%q,"apiKey":%q}`, f.server.URL+"/api", sourceKey)), 200)
	decodeResponse[sources.TestResult](t, f.request(t, "GET", "/api/v1/source/test", ""), 200)
	registered := decodeResponse[struct {
		Movie catalog.Movie `json:"movie"`
	}](t, f.request(t, "POST", "/api/v1/catalog/movies", `{"tmdbId":42,"language":"en-US"}`), 201)
	f.movie = registered.Movie
	return f
}
func (f *releaseFixture) path(extra string) string {
	return fmt.Sprintf("/api/v1/releases?movieId=%d%s", f.movie.ID, extra)
}

func TestReleaseSearchNormalizationProfileAndOfflineRestart(t *testing.T) {
	f := newReleaseFixture(t)
	before := f.detailCalls.Load()
	f.mode.Store(503) // Source search must use saved movie identity without TMDB.
	response := f.request(t, "GET", f.path("&category=2040"), "")
	page := decodeResponse[sources.ReleasePage](t, response, 200)
	query := <-f.queries
	if query.Get("t") != "movie" || query.Get("tmdbid") != "42" || query.Get("q") != "" || query.Get("limit") != "25" || query.Get("cat") != "2040" || query.Get("extended") != "1" || query.Get("apikey") != sourceKey {
		t.Fatal("wrong source query", query.Get("t"), query.Get("tmdbid"))
	}
	if len(page.Items) != 3 || page.Mode != "movie-tmdb" || page.SourceRevision != 1 || page.ProfileRevision != 1 || page.Total == nil || *page.Total != 3 || page.HasMore {
		t.Fatal(page)
	}
	first := page.Items[0]
	if first.Status != "match" || !first.PreferredResolution || first.Format == nil || *first.Format != "remux" || first.SizeBytes == nil || *first.SizeBytes != 1234 || first.Seeders == nil || *first.Seeders != 12 || first.AudioPreferenceRank == nil || *first.AudioPreferenceRank != 2 || first.PublishedAt == nil {
		t.Fatal(first)
	}
	if page.Items[1].Status != "mismatch" || page.Items[2].Status != "unknown" || len(page.Items[2].AudioLanguages) != 0 || page.Items[2].Resolution != nil {
		t.Fatal(page.Items)
	}
	for _, private := range []string{sourceKey, "private-guid", "do-not-fetch", "apikey="} {
		if strings.Contains(response.Body.String(), private) {
			t.Fatal("private source details exposed")
		}
	}
	if f.detailCalls.Load() != before {
		t.Fatal("search contacted metadata provider")
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
	after := decodeResponse[sources.ReleasePage](t, f.request(t, "GET", f.path(""), ""), 200)
	if after.Items[0].ID != first.ID {
		t.Fatal("unstable observation identity across restart")
	}
}

func TestReleaseSearchCapabilityFallbackAndPagination(t *testing.T) {
	f := newReleaseFixture(t)
	f.caps.Store(`<caps><limits max="1"/><searching><search available="yes" supportedParams="q"/><movie-search available="no"/></searching></caps>`)
	decodeResponse[sources.TestResult](t, f.request(t, "GET", "/api/v1/source/test", ""), 200)
	f.feed.Store(`<rss><channel><item><title>Other movie</title><link>http://never-follow.invalid/1</link></item></channel></rss>`)
	page := decodeResponse[sources.ReleasePage](t, f.request(t, "GET", f.path("&limit=50"), ""), 200)
	query := <-f.queries
	if query.Get("t") != "search" || query.Get("q") != "Original Movie 2024" || query.Get("limit") != "1" || query.Get("year") != "" || page.Limit != 1 || !page.HasMore || page.Total != nil {
		t.Fatal(query, page)
	}
	f.feed.Store(`<rss><channel/></rss>`)
	page = decodeResponse[sources.ReleasePage](t, f.request(t, "GET", f.path("&offset=1"), ""), 200)
	if page.HasMore || len(page.Items) != 0 {
		t.Fatal(page)
	}
	<-f.queries
	f.caps.Store(`<caps><searching><movie-search available="yes" supportedParams="q,year"/></searching></caps>`)
	decodeResponse[sources.TestResult](t, f.request(t, "GET", "/api/v1/source/test", ""), 200)
	page = decodeResponse[sources.ReleasePage](t, f.request(t, "GET", f.path(""), ""), 200)
	query = <-f.queries
	if page.Mode != "movie-query" || query.Get("q") != "Original Movie" || query.Get("year") != "2024" {
		t.Fatal(query, page)
	}
	f.caps.Store(`<caps><searching><movie-search available="yes" supportedParams="imdbid"/></searching></caps>`)
	decodeResponse[sources.TestResult](t, f.request(t, "GET", "/api/v1/source/test", ""), 200)
	if r := f.request(t, "GET", f.path(""), ""); r.Code != 409 || !strings.Contains(r.Body.String(), "source-search-unsupported") {
		t.Fatal(r.Code, r.Body.String())
	}
}

func TestReleaseSearchValidationAndClassifiedFailures(t *testing.T) {
	f := newReleaseFixture(t)
	for _, extra := range []string{"&limit=0", "&limit=51", "&offset=10001", "&category=999", "&category=", "&limit=2&limit=3", "&unknown=1"} {
		r := f.request(t, "GET", f.path(extra), "")
		if r.Code != 400 && r.Code != 422 {
			t.Fatal(extra, r.Code)
		}
	}
	if r := f.request(t, "GET", "/api/v1/releases?movieId=99999", ""); r.Code != 404 {
		t.Fatal(r.Code)
	}
	if r := f.request(t, "GET", f.path(""), "{}"); r.Code != 400 {
		t.Fatal(r.Code)
	}
	if f.calls.Load() != 0 {
		t.Fatal("invalid input contacted source")
	}
	for _, entry := range []struct {
		feed   string
		status int
		code   string
	}{
		{`<error code="101" description="dummy-source-key"/>`, 503, "source-authentication-failed"},
		{`<rss><channel>`, 502, "source-invalid-response"},
		{`<html>dummy-source-key</html>`, 502, "source-invalid-response"},
	} {
		f.feed.Store(entry.feed)
		r := f.request(t, "GET", f.path(""), "")
		if r.Code != entry.status || !strings.Contains(r.Body.String(), entry.code) || strings.Contains(r.Body.String(), sourceKey) {
			t.Fatal(r.Code, r.Body.String())
		}
	}
	for _, entry := range []struct {
		status int32
		code   int
		error  string
	}{{401, 503, "source-authentication-failed"}, {500, 503, "source-unavailable"}, {429, 429, "source-rate-limited"}} {
		f.status.Store(entry.status)
		before := f.calls.Load()
		r := f.request(t, "GET", f.path(""), "")
		if r.Code != entry.code || !strings.Contains(r.Body.String(), entry.error) {
			t.Fatal(r.Code, r.Body.String())
		}
		attempts := int32(1)
		if entry.status == 500 {
			attempts = 4
		}
		if f.calls.Load()-before != attempts {
			t.Fatal("wrong retry count")
		}
		if entry.status == 429 {
			r = f.request(t, "GET", f.path(""), "")
			if r.Code != 429 || r.Header().Get("Retry-After") == "" || f.calls.Load()-before != 1 {
				t.Fatal("cooldown not honored")
			}
		}
	}
}

func TestReleaseSearchRejectsStaleSourceOrProfileAndExcludesOperations(t *testing.T) {
	for _, change := range []string{"source", "profile"} {
		t.Run(change, func(t *testing.T) {
			f := newReleaseFixture(t)
			f.block.Store(true)
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() { done <- f.request(t, "GET", f.path(""), "") }()
			select {
			case <-f.queries:
			case <-time.After(time.Second):
				t.Fatal("search never reached source")
			}
			if r := f.request(t, "GET", "/api/v1/source/test", ""); r.Code != 409 {
				t.Fatal("test overlapped search", r.Code)
			}
			if r := f.request(t, "GET", f.path(""), ""); r.Code != 409 {
				t.Fatal("search overlapped", r.Code)
			}
			if change == "source" {
				r := f.request(t, "PUT", "/api/v1/source", fmt.Sprintf(`{"revision":1,"name":"Renamed","url":%q}`, f.server.URL+"/api"))
				if r.Code != 200 {
					t.Fatal(r.Body.String())
				}
			} else {
				profile, err := f.store.GetSettings(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				profile.Profile.PreferredResolution = "720p"
				_, err = f.store.UpdateSettings(context.Background(), profile.Revision, settings.Input{Directories: profile.Directories, Profile: profile.Profile})
				if err != nil {
					t.Fatal(err)
				}
			}
			close(f.release)
			select {
			case r := <-done:
				if r.Code != 409 || !strings.Contains(r.Body.String(), map[string]string{"source": "source-changed", "profile": "preferences-changed"}[change]) {
					t.Fatal(r.Code, r.Body.String())
				}
			case <-time.After(time.Second):
				t.Fatal("search did not return")
			}
		})
	}
}

func TestReleaseSearchRequiresConfiguredTestedSourceAndSetup(t *testing.T) {
	f := newCatalogFixture(t, false)
	if r := f.request(t, "GET", "/api/v1/releases?movieId=1", ""); r.Code != 409 {
		t.Fatal(r.Code)
	}
	f = newCatalogFixture(t, true)
	registered := decodeResponse[struct {
		Movie catalog.Movie `json:"movie"`
	}](t, f.request(t, "POST", "/api/v1/catalog/movies", `{"tmdbId":42,"language":"en-US"}`), 201)
	path := fmt.Sprintf("/api/v1/releases?movieId=%d", registered.Movie.ID)
	if r := f.request(t, "GET", path, ""); r.Code != 503 || !strings.Contains(r.Body.String(), "source-unconfigured") {
		t.Fatal(r.Code, r.Body.String())
	}
	decodeResponse[sources.Public](t, f.request(t, "PUT", "/api/v1/source", `{"revision":0,"name":"Source","url":"http://localhost:1/api","clearApiKey":true}`), 200)
	if r := f.request(t, "GET", path, ""); r.Code != 409 || !strings.Contains(r.Body.String(), "source-capabilities-required") {
		t.Fatal(r.Code, r.Body.String())
	}
}
