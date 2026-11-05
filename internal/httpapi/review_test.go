package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/library"
	"github.com/lekramon/piratarr/internal/settings"
	"github.com/lekramon/piratarr/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func reviewFixture(t *testing.T) (*catalogFixture, string, int64, library.Entry) {
	t.Helper()
	f := newCatalogFixture(t, true)
	config, err := f.store.GetSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(config.Directories.MovieDirectory, "Test.Movie.2024.mkv")
	if err = os.WriteFile(path, []byte("video fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	scan := decodeResponse[library.Snapshot](t, f.request(t, "POST", "/api/v1/library/scan", `{}`), 200)
	registered := decodeResponse[struct {
		Movie catalog.Movie `json:"movie"`
	}](t, f.request(t, "POST", "/api/v1/catalog/movies", `{"tmdbId":42}`), 201)
	f.handler.library = library.New(f.store, f.data, func(context.Context, *os.File) error { return nil })
	return f, path, registered.Movie.ID, scan.Items[0]
}
func reviewFile(t *testing.T, f *catalogFixture, entry library.Entry, action string, movie int64) library.Entry {
	t.Helper()
	input := library.Review{Revision: entry.Revision, Action: action}
	if action == "associate" {
		input.MovieID = &movie
		input.ConfirmIdentity = true
	}
	body, _ := json.Marshal(input)
	return decodeResponse[library.Entry](t, f.request(t, "POST", fmt.Sprintf("/api/v1/library/files/%d/review", entry.ID), string(body)), 200)
}
func TestReviewAssociationRecoveryIgnoreAndInvalidation(t *testing.T) {
	f, path, movieID, entry := reviewFixture(t)
	linked := reviewFile(t, f, entry, "associate", movieID)
	if linked.ReviewState != "associated" || linked.MovieID == nil || *linked.MovieID != movieID || linked.VerifiedAt == nil {
		t.Fatal(linked)
	}
	f.store.Close()
	var reopenErr error
	f.store, reopenErr = storage.Open(context.Background(), f.data)
	if reopenErr != nil {
		t.Fatal(reopenErr)
	}
	f.rebind()
	f.handler.library = library.New(f.store, f.data, func(context.Context, *os.File) error { return nil })
	persistedLink := decodeResponse[library.Snapshot](t, f.request(t, "GET", "/api/v1/library/files", ""), 200).Items[0]
	if persistedLink.ReviewState != "associated" || persistedLink.MovieID == nil {
		t.Fatal(persistedLink)
	}
	movie := decodeResponse[catalog.Movie](t, f.request(t, "GET", fmt.Sprintf("/api/v1/catalog/movies/%d", movieID), ""), 200)
	if !movie.HasMedia {
		t.Fatal("associated movie not available")
	}
	config, rootErr := f.store.GetSettings(context.Background())
	if rootErr != nil {
		t.Fatal(rootErr)
	}
	changedRoot := settings.Input{Directories: config.Directories, Profile: config.Profile}
	changedRoot.Directories.MovieDirectory = t.TempDir()
	next, rootErr := f.store.UpdateSettings(context.Background(), config.Revision, changedRoot)
	if rootErr != nil {
		t.Fatal(rootErr)
	}
	oldRootMovie := decodeResponse[catalog.Movie](t, f.request(t, "GET", fmt.Sprintf("/api/v1/catalog/movies/%d", movieID), ""), 200)
	if oldRootMovie.HasMedia {
		t.Fatal("old-root media advertised")
	}
	if _, rootErr = f.store.UpdateSettings(context.Background(), next.Revision, settings.Input{Directories: config.Directories, Profile: config.Profile}); rootErr != nil {
		t.Fatal(rootErr)
	}
	stable := decodeResponse[library.Snapshot](t, f.request(t, "POST", "/api/v1/library/scan", `{}`), 200).Items[0]
	if stable.ReviewState != "associated" || *stable.VerifiedAt != *linked.VerifiedAt {
		t.Fatal(stable)
	}
	stale := f.request(t, "POST", fmt.Sprintf("/api/v1/library/files/%d/review", entry.ID), fmt.Sprintf(`{"revision":%d,"action":"ignore"}`, entry.Revision))
	if stale.Code != 409 {
		t.Fatal(stale.Code)
	}
	ignored := reviewFile(t, f, stable, "ignore", 0)
	if ignored.ReviewState != "ignored" || ignored.MovieID != nil {
		t.Fatal(ignored)
	}
	reopened := reviewFile(t, f, ignored, "reopen", 0)
	linked = reviewFile(t, f, reopened, "associate", movieID)
	if err := os.WriteFile(path, []byte("a changed video fixture with a new size"), 0600); err != nil {
		t.Fatal(err)
	}
	changed := decodeResponse[library.Snapshot](t, f.request(t, "POST", "/api/v1/library/scan", `{}`), 200).Items[0]
	if changed.ReviewState != "pending" || changed.MovieID != nil || changed.VerifiedAt != nil {
		t.Fatal(changed)
	}
	movie = decodeResponse[catalog.Movie](t, f.request(t, "GET", fmt.Sprintf("/api/v1/catalog/movies/%d", movieID), ""), 200)
	if movie.HasMedia {
		t.Fatal("changed media still advertised")
	}
	linked = reviewFile(t, f, changed, "associate", movieID)
	os.Remove(path)
	missing := decodeResponse[library.Snapshot](t, f.request(t, "POST", "/api/v1/library/scan", `{}`), 200).Items[0]
	if missing.ReviewState != "pending" || missing.Status != "missing" {
		t.Fatal(missing)
	}
	ignored = reviewFile(t, f, missing, "ignore", 0)
	os.WriteFile(path, []byte("new video"), 0600)
	ignored = decodeResponse[library.Snapshot](t, f.request(t, "POST", "/api/v1/library/scan", `{}`), 200).Items[0]
	if ignored.ReviewState != "ignored" {
		t.Fatal(ignored)
	}
	f.store.Close()
	var err error
	f.store, err = storage.Open(context.Background(), f.data)
	if err != nil {
		t.Fatal(err)
	}
	f.rebind()
	persisted := decodeResponse[library.Snapshot](t, f.request(t, "GET", "/api/v1/library/files", ""), 200).Items[0]
	if persisted.ReviewState != "ignored" || persisted.ID != entry.ID {
		t.Fatal(persisted)
	}
	bytes, _ := os.ReadFile(path)
	if string(bytes) != "new video" {
		t.Fatal("review changed original")
	}
}
func TestReviewValidationProbeFailureAndIdentityChanges(t *testing.T) {
	f, path, movieID, entry := reviewFixture(t)
	route := fmt.Sprintf("/api/v1/library/files/%d/review", entry.ID)
	for _, body := range []string{`null`, `{"revision":1,"action":"associate","movieId":1}`, `{"revision":1,"action":"ignore","movieId":1}`, `{"revision":1,"revision":1,"action":"ignore"}`, `{"revision":1,"action":"bad"}`, `{"revision":1,"action":"ignore","extra":true}`} {
		response := f.request(t, "POST", route, body)
		if response.Code != 400 && response.Code != 422 {
			t.Fatal(body, response.Code)
		}
	}
	body := fmt.Sprintf(`{"revision":%d,"action":"associate","movieId":%d,"confirmIdentity":true}`, entry.Revision, movieID)
	for _, failure := range []struct {
		err  error
		code int
	}{{library.ErrInvalidVideo, 422}, {library.ErrProbeUnavailable, 503}, {library.ErrProbeTimeout, 504}} {
		f.handler.library = library.New(f.store, f.data, func(context.Context, *os.File) error { return failure.err })
		r := f.request(t, "POST", route, body)
		if r.Code != failure.code || strings.Contains(r.Body.String(), path) {
			t.Fatal(r.Code, r.Body.String())
		}
	}
	f.handler.library = library.New(f.store, f.data, func(context.Context, *os.File) error {
		os.WriteFile(path, []byte("changed while probing file"), 0600)
		return nil
	})
	r := f.request(t, "POST", route, body)
	if r.Code != 409 {
		t.Fatal(r.Code, r.Body.String())
	}
	result := decodeResponse[library.Snapshot](t, f.request(t, "GET", "/api/v1/library/files", ""), 200)
	if result.Items[0].ReviewState != "pending" {
		t.Fatal(result)
	}
}
func TestReviewSettingsChangeDuringVerification(t *testing.T) {
	f, _, movieID, entry := reviewFixture(t)
	f.handler.library = library.New(f.store, f.data, func(ctx context.Context, _ *os.File) error {
		config, err := f.store.GetSettings(ctx)
		if err != nil {
			return err
		}
		_, err = f.store.UpdateSettings(ctx, config.Revision, settings.Input{Directories: config.Directories, Profile: config.Profile})
		return err
	})
	body := fmt.Sprintf(`{"revision":%d,"action":"associate","movieId":%d,"confirmIdentity":true}`, entry.Revision, movieID)
	r := f.request(t, "POST", fmt.Sprintf("/api/v1/library/files/%d/review", entry.ID), body)
	if r.Code != 409 || !strings.Contains(r.Body.String(), "settings-changed") {
		t.Fatal(r.Code, r.Body.String())
	}
}
