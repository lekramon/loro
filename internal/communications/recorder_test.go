package communications

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func event() Event {
	return Event{Integration: "source", Operation: "source.request", Phase: "response.received", Status: Ptr(200)}
}
func TestBoundedHistoryCursorsFiltersAndRestart(t *testing.T) {
	r := New("")
	start := r.Read("", 200, "", "").Cursor
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Go(func() {
			for j := 0; j < 510; j++ {
				r.Add(event())
			}
		})
	}
	wg.Wait()
	page := r.Read(start, 200, "source", "")
	if len(r.items) != Capacity || !page.Truncated || len(page.Entries) != 200 {
		t.Fatal("unbounded or missing history", len(r.items), page.Truncated, len(page.Entries))
	}
	seen := map[string]bool{}
	for len(page.Entries) > 0 {
		for _, e := range page.Entries {
			if seen[e.ID] {
				t.Fatal("duplicate event", e.ID)
			}
			seen[e.ID] = true
		}
		page = r.Read(page.Cursor, 200, "source", "")
	}
	if len(seen) != Capacity {
		t.Fatal("cursor skipped events", len(seen))
	}
	if got := r.Read(page.Cursor, 200, "metadata", ""); len(got.Entries) != 0 || got.Cursor != page.Cursor {
		t.Fatal("empty filter lost cursor")
	}
	if !r.Read("0000000000000000:0", 10, "", "").Reset {
		t.Fatal("old session not reset")
	}
	if !r.Read(fmt.Sprintf("%s:%d", r.session, r.sequence+1), 10, "", "").Reset {
		t.Fatal("future cursor not reset")
	}
	for _, bad := range []string{"", "abcd:1", r.session + ":+1", r.session + ":-1", r.session + ":1:2", r.session + ":18446744073709551616", r.session + ":" + strings.Repeat("0", 24)} {
		if _, _, err := ParseCursor(bad); err == nil {
			t.Fatal("accepted cursor", bad)
		}
	}

	root := t.TempDir()
	disk := New(root)
	disk.fileLimit = 2048
	for i := 0; i < 30; i++ {
		disk.Add(event())
	}
	files, err := filepath.Glob(filepath.Join(root, "logs", "communications.*.ndjson"))
	if err != nil || len(files) != 3 {
		t.Fatal(files, err)
	}
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil || info.Size() > disk.fileLimit {
			t.Fatal("rotation exceeded bound", err)
		}
	}
	old := disk.Read("", 200, "", "")
	restored := New(root).Read(old.Cursor, 200, "", "")
	if len(restored.Entries) == 0 || !restored.Reset || !restored.StorageAvailable {
		t.Fatal("history lost at restart")
	}
	for _, e := range restored.Entries {
		if !eventPattern.MatchString(e.ID) {
			t.Fatal("invalid restored identity")
		}
	}
	match := restored.Entries[0]
	filtered := New(root).Read("", 200, "source", match.RequestID)
	if len(filtered.Entries) != 1 || filtered.Entries[0].ID != match.ID {
		t.Fatal("correlation filter failed")
	}
}
func TestPersistenceFailureAndInvalidStoredRecords(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "logs"), []byte("owned sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	r := New(root)
	r.Add(event())
	p := r.Read("", 10, "", "")
	if p.StorageAvailable || len(p.Entries) != 1 {
		t.Fatal("persistence failure hid memory history")
	}
	data, _ := os.ReadFile(filepath.Join(root, "logs"))
	if string(data) != "owned sentinel" {
		t.Fatal("unrelated file modified")
	}
	dir := t.TempDir()
	r = New(dir)
	r.Add(event())
	good := r.Read("", 10, "", "").Entries[0]
	old := good
	old.Timestamp = time.Now().Add(-8 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)
	invalid := good
	invalid.ID = "private-path-canary"
	invalid.Operation = "https://secret.example/"
	f, err := os.OpenFile(r.path(0), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []Event{old, invalid} {
		line, _ := json.Marshal(e)
		fmt.Fprintln(f, string(line))
	}
	fmt.Fprint(f, "{truncated")
	f.Close()
	restored := New(dir).Read("", 10, "", "")
	if len(restored.Entries) != 1 || restored.Entries[0].ID != good.ID {
		t.Fatal("unsafe stored record accepted")
	}
	for _, bad := range []Event{{Integration: "secret"}, {Integration: "source", Operation: "/private/path", Phase: "request.started"}} {
		r.Add(bad)
	}
	if len(r.Read("", 10, "", "").Entries) != 1 {
		t.Fatal("invalid schema accepted")
	}
}
