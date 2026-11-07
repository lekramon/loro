package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lekramon/piratarr/internal/acquisitions"
	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/sources"
)

func TestNativeSourceConfigurationAndMissingBrowser(t *testing.T) {
	f := newCatalogFixture(t, true)
	t.Setenv("LOURO_BROWSER_PYTHON", "louro-deliberately-absent-browser")
	for _, input := range []string{`"kind":"unknown","url":"http://localhost"`, `"kind":"1337x","url":"http://localhost/path"`, `"kind":"1337x","url":"http://localhost","apiKey":"private"`} {
		r := f.request(t, "PUT", "/api/v1/source", `{"revision":0,"name":"Source",`+input+`}`)
		if r.Code != 422 {
			t.Fatal(r.Code, r.Body.String())
		}
	}
	saved := decodeResponse[sources.Public](t, f.request(t, "PUT", "/api/v1/source", `{"revision":0,"kind":"1337x","name":"Direct","url":"http://localhost","clearApiKey":true}`), 200)
	if saved.Kind != "1337x" || saved.HasAPIKey {
		t.Fatal(saved)
	}
	response := f.request(t, "GET", "/api/v1/source/test", "")
	if response.Code != 503 || !strings.Contains(response.Body.String(), "source-browser-unavailable") {
		t.Fatal(response.Code, response.Body.String())
	}
}

func TestInstalledNativeSourceSearchAndPreparation(t *testing.T) {
	if os.Getenv("LOURO_TEST_BROWSER") != "1" {
		t.Skip("explicit installed-browser API test")
	}
	f := newCatalogFixture(t, true)
	var details atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(204)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/torrent/") {
			details.Add(1)
			fmt.Fprintf(w, `<html><body><a href="magnet:?xt=urn:btih:%s">Magnet</a></body></html>`, selectionHash)
			return
		}
		if r.URL.Path != "/cat/Movies/1/" && r.URL.Path != "/category-search/Original Movie 2024/Movies/1/" {
			t.Errorf("unexpected direct path %q", r.URL.Path)
		}
		fmt.Fprint(w, `<html><body><table class="table-list"><tr><td class="coll-1"><a href="/sub/42/">Movie</a><a href="/torrent/123/Example/">Example.2024.1080p.Remux.mkv</a></td><td class="coll-2">7</td><td class="coll-4">1.5 GB</td></tr></table></body></html>`)
	}))
	defer server.Close()
	decodeResponse[sources.Public](t, f.request(t, "PUT", "/api/v1/source", fmt.Sprintf(`{"revision":0,"kind":"1337x","name":"Direct","url":%q,"clearApiKey":true}`, server.URL)), 200)
	decodeResponse[sources.TestResult](t, f.request(t, "GET", "/api/v1/source/test", ""), 200)
	registered := decodeResponse[struct {
		Movie catalog.Movie `json:"movie"`
	}](t, f.request(t, "POST", "/api/v1/catalog/movies", `{"tmdbId":42}`), 201)
	response := f.request(t, "GET", fmt.Sprintf("/api/v1/releases?movieId=%d", registered.Movie.ID), "")
	page := decodeResponse[sources.ReleasePage](t, response, 200)
	if len(page.Items) != 1 || page.Mode != "1337x-query" || page.Limit != 20 || page.Items[0].Seeders == nil || *page.Items[0].Seeders != 7 || page.Items[0].SizeBytes == nil || *page.Items[0].SizeBytes != 1610612736 {
		t.Fatal(page)
	}
	if details.Load() != 0 || strings.Contains(response.Body.String(), "/torrent/") || strings.Contains(response.Body.String(), "magnet:") {
		t.Fatal("search fetched or exposed private details")
	}
	selected := decodeResponse[struct {
		Acquisition acquisitions.Acquisition `json:"acquisition"`
	}](t, f.request(t, "POST", "/api/v1/acquisitions", fmt.Sprintf(`{"observationId":%q,"idempotencyKey":"native-source-selection","confirmOverride":true}`, page.Items[0].ObservationID)), 201)
	prepared := decodeResponse[acquisitions.Acquisition](t, f.request(t, "POST", "/api/v1/acquisitions/"+selected.Acquisition.ID+"/prepare", fmt.Sprintf(`{"revision":%d}`, selected.Acquisition.Revision)), 200)
	if prepared.State != "ready" || prepared.InfoHash == nil || *prepared.InfoHash != selectionHash || details.Load() != 1 {
		t.Fatal(prepared)
	}
	record, err := f.store.GetAcquisition(context.Background(), prepared.ID)
	if err != nil || !strings.HasPrefix(record.Reference, "magnet:?") || record.Kind == nil || *record.Kind != "magnet" {
		t.Fatal("prepared reference not usable for submission", err)
	}
}
