package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lekramon/piratarr/internal/storage"
)

func TestSeriesCatalogAPIUsesAdditiveSQLiteIdentity(t *testing.T) {
	store, err := storage.Open(context.Background(), t.TempDir()); if err != nil { t.Fatal(err) }; defer store.Close()
	handler := New(store, fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}}, "127.0.0.1:8080")
	request := func(method, path, body string) *httptest.ResponseRecorder { r:=httptest.NewRequest(method,"http://localhost:8080"+path,strings.NewReader(body));if method!="GET"{r.Header.Set("X-Piratarr-Request","1");r.Header.Set("Content-Type","application/json")};w:=httptest.NewRecorder();handler.ServeHTTP(w,r);return w }
	created:=request(http.MethodPost,"/api/v1/series",`{"provider":"tmdb","providerId":1399,"title":"Test Series","anime":false}`);if created.Code!=201{t.Fatalf("create: %d %s",created.Code,created.Body.String())}
	listed:=request(http.MethodGet,"/api/v1/series?limit=10&offset=0","");if listed.Code!=200||!strings.Contains(listed.Body.String(),"Test Series"){t.Fatalf("list: %d %s",listed.Code,listed.Body.String())}
	duplicate:=request(http.MethodPost,"/api/v1/series",`{"provider":"tmdb","providerId":1399,"title":"Duplicate"}`);if duplicate.Code!=409{t.Fatalf("duplicate: %d %s",duplicate.Code,duplicate.Body.String())}
	seasons:=request(http.MethodGet,"/api/v1/series/1/seasons","");if seasons.Code!=200{t.Fatalf("seasons: %d %s",seasons.Code,seasons.Body.String())}
}
