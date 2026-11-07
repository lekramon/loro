package sources

import (
	"context"
	"strconv"
	"testing"

	"github.com/lekramon/piratarr/internal/browser"
)

func TestNativePaginationPreservesRawMixedCategoryPositions(t *testing.T) {
	config := Config{Public: Public{URL: "https://example.test", Revision: 1}}
	result := browser.Result{RawCount: 40, HasMore: true}
	for i := 0; i < 40; i += 2 {
		result.Rows = append(result.Rows, browser.Row{Title: "Movie.1080p", Path: "/torrent/" + strconv.Itoa(i+1) + "/Title/", Size: "1.5 GB", Seeders: "?", Position: i})
	}
	first, err := normalize1337x(config, result, 20, 0)
	second, secondErr := normalize1337x(config, result, 20, 20)
	if err != nil || secondErr != nil || len(first.Items) != 10 || len(second.Items) != 10 || first.NextOffset == nil || *first.NextOffset != 20 || second.NextOffset == nil || *second.NextOffset != 40 {
		t.Fatal(first, second, err, secondErr)
	}
	seen := map[string]bool{}
	for _, c := range first.Items {
		seen[c.ID] = true
		if c.SizeBytes == nil || *c.SizeBytes != 1610612736 || c.Seeders != nil || len(c.AudioLanguages) != 0 {
			t.Fatal(c)
		}
	}
	for _, c := range second.Items {
		if seen[c.ID] {
			t.Fatal("mixed-category offset duplicated a result")
		}
	}
	result = browser.Result{RawCount: 1, Rows: []browser.Row{{Title: "Movie", Path: "/torrent/1/Title/", Position: 0}}}
	last, err := normalize1337x(config, result, 20, 40)
	if err != nil || last.HasMore || last.NextOffset != nil || len(last.Items) != 1 {
		t.Fatal(last, err)
	}
}

func TestNativeDetailsRejectCrossOriginBeforeBrowser(t *testing.T) {
	for _, ref := range []string{"louro-1337x:https://other.test/torrent/1/Title/", "louro-1337x:https://example.test/not-details/", "louro-1337x:file:///torrent/1/Title/", "louro-1337x:https://example.test/torrent/1/Title/?token=secret"} {
		if _, code := Resolve1337x(context.Background(), "https://example.test", ref); code != "unsupported-reference" {
			t.Fatal("unsafe reference accepted", code)
		}
	}
}
