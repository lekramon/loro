package importlists

import (
	"strings"
	"testing"
)

func TestCSVAdaptersParseStableIdentityAndRejectDuplicateRows(t *testing.T) {
	letterboxd, err := parse("letterboxd-csv", "Date,Name,Year,Letterboxd URI\n2026-01-01,Inception,2010,https://letterboxd.com/film/inception/\n2026-01-02,Inception,2010,https://letterboxd.com/film/inception/\n")
	if err != nil || len(letterboxd) != 2 || letterboxd[0].SourceItemKey != "https://letterboxd.com/film/inception/" || letterboxd[0].Year == nil || *letterboxd[0].Year != 2010 || letterboxd[1].Status != "invalid" || letterboxd[1].ErrorCode != "duplicate-row" {
		t.Fatal(letterboxd, err)
	}
	imdb, err := parse("imdb-csv", "Const,Title,Year,Title Type\ntt1375666,Inception,2010,movie\n")
	if err != nil || len(imdb) != 1 || imdb[0].SourceItemKey != "tt1375666" || imdb[0].Title != "Inception" {
		t.Fatal(imdb, err)
	}
}

func TestCSVAdaptersEnforceHeadersAndRowBound(t *testing.T) {
	if _, err := parse("letterboxd-csv", "Title,Year\nInception,2010\n"); err == nil {
		t.Fatal("accepted an export without Letterboxd headers")
	}
	var csv strings.Builder
	csv.WriteString("Const,Title,Year\n")
	for index := 0; index <= MaxRows; index++ {
		csv.WriteString("tt" + strings.Repeat("0", 7) + ",Movie,2020\n")
	}
	if _, err := parse("imdb-csv", csv.String()); err == nil {
		t.Fatal("accepted more than the bounded row count")
	}
	oversized, err := parse("imdb-csv", "Const,Title,Year\ntt1,"+strings.Repeat("x", 301)+",2020\n")
	if err != nil || len(oversized) != 1 || oversized[0].Status != "invalid" {
		t.Fatal("accepted an oversized title field", oversized, err)
	}
}
