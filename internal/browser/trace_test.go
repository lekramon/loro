package browser

import (
	"context"
	"encoding/json"
	"github.com/lekramon/piratarr/internal/communications"
	"strings"
	"testing"
)

func TestBrowserTraceAcceptsOnlyBoundedProtocol(t *testing.T) {
	r := communications.New(t.TempDir())
	ctx := communications.Correlate(communications.WithRecorder(context.Background(), r))
	w := &traceWriter{ctx: ctx, operation: "browser.search"}
	for _, line := range []string{"raw-library-secret\n", strings.Repeat("x", 5000) + "\n", `{"phase":"secret-phase"}` + "\n", `{"phase":"browser.response","status":200,"kind":"https://secret.invalid","url":"url-canary","cookie":"cookie-canary"}` + "\n", `{"phase":"browser.failed","code":"password-canary"}` + "\n"} {
		for i := 0; i < len(line); i += 7 {
			end := min(i+7, len(line))
			_, _ = w.Write([]byte(line[i:end]))
		}
	}
	p := r.Read("", 10, "", "")
	if len(p.Entries) != 2 || p.Entries[0].Operation != "browser.search" || p.Entries[1].Code != nil {
		t.Fatal("untrusted protocol accepted")
	}
	data, _ := json.Marshal(p)
	for _, secret := range []string{"secret", "canary", "invalid"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("private browser output retained")
		}
	}
}
func TestBrowserMissingRuntimeRecordsFailureAndCleanup(t *testing.T) {
	t.Setenv("LOURO_BROWSER_PYTHON", t.TempDir()+"/missing")
	r := communications.New("")
	ctx := communications.WithRecorder(context.Background(), r)
	Fetch(ctx, "http://secret.invalid/", "search")
	p := r.Read("", 10, "", "")
	if len(p.Entries) != 3 || p.Entries[1].Phase != "operation.failed" || *p.Entries[1].Code != "source-browser-unavailable" || p.Entries[2].Phase != "browser.stopped" {
		t.Fatal("early failure was invisible", p)
	}
}
