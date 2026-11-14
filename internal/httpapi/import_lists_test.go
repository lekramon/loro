package httpapi

import (
	"context"
	"encoding/json"
	"testing"
	"testing/fstest"

	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/importlists"
	"github.com/lekramon/piratarr/internal/storage"
)

type importListProvider struct{}

func (importListProvider) Configured() bool { return true }
func (importListProvider) Search(_ context.Context, query string, _, _ int, language string) (catalog.SearchResult, error) {
	result := catalog.SearchResult{Page: 1, TotalPages: 1, Results: []catalog.Metadata{}}
	switch query {
	case "Inception":
		result.Results = []catalog.Metadata{listMetadata(1, "Inception", 2010, language)}
	case "Existing":
		result.Results = []catalog.Metadata{listMetadata(2, "Existing", 2020, language)}
	case "Ambiguous":
		result.Results = []catalog.Metadata{listMetadata(3, "Ambiguous", 2021, language), listMetadata(4, "Ambiguous", 2021, language)}
	}
	result.TotalResults = len(result.Results)
	return result, nil
}
func (importListProvider) Details(_ context.Context, id int64, language string) (catalog.Metadata, error) {
	switch id {
	case 1:
		return listMetadata(1, "Inception", 2010, language), nil
	case 2:
		return listMetadata(2, "Existing", 2020, language), nil
	}
	return catalog.Metadata{}, catalog.ErrNotFound
}
func (importListProvider) Image(context.Context, string, string) ([]byte, error) {
	return nil, catalog.ErrNotFound
}
func listMetadata(id int64, title string, year int, language string) catalog.Metadata {
	return catalog.Metadata{TMDBID: id, Title: title, OriginalTitle: title, Year: &year, Genres: []string{}, MetadataLanguage: language}
}

func TestImportListAPIPreviewsCommitsAndNeverDeletesMissingItems(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.CreateSettings(ctx, validSetup(t)); err != nil {
		t.Fatal(err)
	}
	catalogService := catalog.New(store, importListProvider{}, t.TempDir())
	if _, created, registerErr := catalogService.Register(ctx, 2, "en-US"); registerErr != nil || !created {
		t.Fatal(created, registerErr)
	}
	handler := New(store, fstest.MapFS{}, "127.0.0.1:8080", catalogService)
	csv := "Date,Name,Year,Letterboxd URI\n2026-01-01,Inception,2010,lb-inception\n2026-01-01,Existing,2020,lb-existing\n2026-01-01,Ambiguous,2021,lb-ambiguous\n2026-01-01,Missing,2022,lb-missing\n"
	previewResponse := setupRequest(t, handler, "POST", "/api/v1/import-lists/preview", encoded(t, map[string]any{"source": "letterboxd-csv", "listKey": "watchlist", "name": "Watchlist", "csv": csv}), false)
	if previewResponse.Code != 200 {
		t.Fatalf("%d %s", previewResponse.Code, previewResponse.Body.String())
	}
	var preview importlists.Preview
	if err = json.Unmarshal(previewResponse.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	statuses := map[string]int{}
	for _, row := range preview.Rows {
		statuses[row.Status]++
	}
	if statuses["addition"] != 1 || statuses["duplicate"] != 1 || statuses["ambiguous"] != 1 || statuses["missing"] != 1 {
		t.Fatal(statuses)
	}
	commitPath := "/api/v1/import-lists/previews/" + preview.ID + "/commit"
	commitResponse := setupRequest(t, handler, "POST", commitPath, encoded(t, map[string]int{"revision": preview.Revision}), false)
	if commitResponse.Code != 200 {
		t.Fatalf("%d %s", commitResponse.Code, commitResponse.Body.String())
	}
	var committed importlists.CommitResult
	_ = json.Unmarshal(commitResponse.Body.Bytes(), &committed)
	if committed.Linked != 2 || committed.Created != 1 || committed.Duplicates != 1 || committed.State != "committed" {
		t.Fatal(committed)
	}
	repeated := setupRequest(t, handler, "POST", commitPath, encoded(t, map[string]int{"revision": preview.Revision}), false)
	var repeatedResult importlists.CommitResult
	_ = json.Unmarshal(repeated.Body.Bytes(), &repeatedResult)
	if repeated.Code != 200 || repeatedResult != committed {
		t.Fatal(repeated.Code, repeatedResult, committed)
	}
	refreshCSV := "Date,Name,Year,Letterboxd URI\n2026-02-01,Inception,2010,lb-inception\n"
	refreshResponse := setupRequest(t, handler, "POST", "/api/v1/import-lists/preview", encoded(t, map[string]any{"source": "letterboxd-csv", "listKey": "watchlist", "name": "Watchlist", "csv": refreshCSV}), false)
	var refresh importlists.Preview
	_ = json.Unmarshal(refreshResponse.Body.Bytes(), &refresh)
	if response := setupRequest(t, handler, "POST", "/api/v1/import-lists/previews/"+refresh.ID+"/commit", encoded(t, map[string]int{"revision": refresh.Revision}), false); response.Code != 200 {
		t.Fatalf("%d %s", response.Code, response.Body.String())
	}
	lists, err := store.ListImportLists(ctx)
	if err != nil || len(lists) != 1 || lists[0].Items != 2 || lists[0].Revision != 2 {
		t.Fatal(lists, err)
	}
	movies, err := store.ListMovies(ctx, 10, 0, "")
	if err != nil || movies.Total != 2 {
		t.Fatal(movies, err)
	}
}
