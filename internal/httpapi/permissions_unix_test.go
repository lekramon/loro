//go:build !windows

package httpapi

import (
	"context"
	"os"
	"testing"
	"testing/fstest"

	"github.com/lekramon/piratarr/internal/storage"
)

func TestSetupRejectsReadOnlyDirectoryMode(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory write restrictions")
	}
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := New(store, fstest.MapFS{}, "127.0.0.1:8080")
	input := validSetup(t)
	if err := os.Chmod(input.Directories.MovieDirectory, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(input.Directories.MovieDirectory, 0700) })
	response := setupRequest(t, h, "POST", "/api/v1/setup", encoded(t, input), false)
	if response.Code != 422 {
		t.Fatalf("read-only directory accepted: %d %s", response.Code, response.Body.String())
	}
}
