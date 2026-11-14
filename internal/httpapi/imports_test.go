package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lekramon/piratarr/internal/storage"
)

func TestImportAPIRequiresExactlyOneMediaIdentity(t *testing.T) {
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := New(store, fstest.MapFS{}, "127.0.0.1:8080")
	const prefix = `{"downloadId":"0123456789abcdef0123456789abcdef","fileId":"Series/S01E01.mkv",`

	for name, body := range map[string]string{
		"missing": prefix + `"revision":1,"conflictPolicy":"skip"}`,
		"both":    prefix + `"movieId":1,"episodeId":2,"revision":1,"conflictPolicy":"skip"}`,
	} {
		t.Run(name, func(t *testing.T) {
			response := setupRequest(t, handler, "POST", "/api/v1/imports", []byte(body), false)
			if response.Code != 422 || !strings.Contains(response.Body.String(), "validation-failed") {
				t.Fatalf("%d %s", response.Code, response.Body.String())
			}
		})
	}

	for name, body := range map[string]string{
		"movie":   prefix + `"movieId":1,"revision":1,"conflictPolicy":"skip"}`,
		"episode": prefix + `"episodeId":2,"revision":1,"conflictPolicy":"skip"}`,
	} {
		t.Run(name, func(t *testing.T) {
			response := setupRequest(t, handler, "POST", "/api/v1/imports", []byte(body), false)
			if response.Code != 404 || !strings.Contains(response.Body.String(), "import-not-found") {
				t.Fatalf("identity was not accepted: %d %s", response.Code, response.Body.String())
			}
		})
	}
}
