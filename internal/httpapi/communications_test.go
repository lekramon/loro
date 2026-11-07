package httpapi

import (
	"encoding/json"
	"github.com/lekramon/piratarr/internal/communications"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCommunicationsExposeSanitizedAttemptsWithoutPollingItself(t *testing.T) {
	f := newSourceFixture(t)
	f.save(t, 0, sourceKey)
	decodeResponse[map[string]any](t, f.request(t, "GET", "/api/v1/source/test", ""), 200)
	r := f.request(t, "GET", "/api/v1/system/communications?limit=200", "")
	if r.Code != 200 {
		t.Fatalf("diagnostics endpoint: %d", r.Code)
	}
	var page struct {
		Entries []struct {
			Integration, Phase, RequestID string
			Status                        *int
		}
		Cursor           string
		StorageAvailable bool
	}
	if json.Unmarshal(r.Body.Bytes(), &page) != nil || len(page.Entries) == 0 || !page.StorageAvailable {
		t.Fatal(r.Body.String())
	}
	found := false
	for _, e := range page.Entries {
		if e.Integration == "source" && e.Phase == "response.received" && e.Status != nil && *e.Status == 200 && len(e.RequestID) == 32 {
			found = true
		}
	}
	if !found {
		t.Fatal("missing outbound source response")
	}
	for _, secret := range []string{sourceKey, "apikey=", "/custom/api", f.server.URL} {
		if strings.Contains(r.Body.String(), secret) {
			t.Fatal("private request data exposed")
		}
	}
	after := f.request(t, "GET", "/api/v1/system/communications?after="+page.Cursor, "")
	var next struct {
		Entries []json.RawMessage
		Cursor  string
	}
	if after.Code != 200 || json.Unmarshal(after.Body.Bytes(), &next) != nil || len(next.Entries) != 0 || next.Cursor != page.Cursor {
		t.Fatal("polling logged itself", after.Body.String())
	}
	for _, query := range []string{"?limit=0", "?integration=unknown", "?after=bad", "?requestId=secret", "?limit=1&limit=2", "?path=private"} {
		result := f.request(t, "GET", "/api/v1/system/communications"+query, "")
		if result.Code != 400 && result.Code != 422 {
			t.Fatal("invalid diagnostics query accepted", query, result.Code)
		}
	}
}

func TestCommunicationsPublicAPIShowsSlowRequestFailureAndRestart(t *testing.T) {
	f := newSourceFixture(t)
	f.save(t, 0, sourceKey)
	f.mode.Store(2)
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- f.request(t, "GET", "/api/v1/source/test", "") }()
	deadline := time.Now().Add(time.Second)
	var started communications.Event
	for time.Now().Before(deadline) {
		p := decodeResponse[communications.Page](t, f.request(t, "GET", "/api/v1/system/communications?integration=source", ""), 200)
		for _, e := range p.Entries {
			if e.Phase == "request.started" {
				started = e
			}
		}
		if started.ID != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if started.ID == "" {
		t.Fatal("slow source request was invisible")
	}
	response := <-done
	if response.Code != 504 {
		t.Fatal(response.Code)
	}
	p := decodeResponse[communications.Page](t, f.request(t, "GET", "/api/v1/system/communications?requestId="+started.RequestID, ""), 200)
	attempts := 0
	found := false
	for _, e := range p.Entries {
		if e.Integration == "source" && e.Phase == "request.started" {
			attempts++
		}
		if e.Integration == "source" && e.Phase == "request.failed" && e.Code != nil && *e.Code == "request-timeout" {
			found = true
		}
	}
	if !found || attempts != 4 {
		t.Fatal("timeout summary or retry attempts missing", p)
	}
	calls := f.calls.Load()
	f.rebind()
	restarted := decodeResponse[communications.Page](t, f.request(t, "GET", "/api/v1/system/communications?after="+p.Cursor, ""), 200)
	if !restarted.Reset || len(restarted.Entries) == 0 || f.calls.Load() != calls {
		t.Fatal("restart lost history or contacted source")
	}
	for _, file := range []string{"communications.0.ndjson", "communications.1.ndjson", "communications.2.ndjson"} {
		data, err := os.ReadFile(filepath.Join(f.data, "logs", file))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{sourceKey, f.server.URL, "apikey=", "/custom/api", f.data} {
			if strings.Contains(string(data), secret) {
				t.Fatal("persisted private data")
			}
		}
	}
	if r := f.request(t, "GET", "/api/v1/system/communications", `{"secret":"body-canary"}`); r.Code != 400 {
		t.Fatal("request body accepted", r.Code)
	}
}

func TestCommunicationsStorageFailureDoesNotFailSource(t *testing.T) {
	f := newSourceFixture(t)
	f.save(t, 0, sourceKey)
	blocked := filepath.Join(t.TempDir(), "blocked-root")
	if err := os.WriteFile(blocked, []byte("sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	f.handler.communications = communications.New(blocked)
	decodeResponse[map[string]any](t, f.request(t, "GET", "/api/v1/source/test", ""), 200)
	p := decodeResponse[communications.Page](t, f.request(t, "GET", "/api/v1/system/communications", ""), 200)
	if p.StorageAvailable || len(p.Entries) == 0 {
		t.Fatal("storage failure hid memory history or claimed persistence")
	}
	data, _ := os.ReadFile(blocked)
	if string(data) != "sentinel" {
		t.Fatal("logging modified unrelated file")
	}
}
