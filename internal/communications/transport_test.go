package communications

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestTransportRecordsStartResponsesAndErrorsWithoutPrivateData(t *testing.T) {
	root := t.TempDir()
	r := New(root)
	ctx := Correlate(WithRecorder(context.Background(), r))
	entered, release := make(chan struct{}), make(chan struct{})
	base := roundTripFunc(func(q *http.Request) (*http.Response, error) {
		close(entered)
		<-release
		return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"7200"}, "Set-Cookie": []string{"cookie-canary"}}, Body: io.NopCloser(strings.NewReader("body-canary")), Request: q}, nil
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", "http://user-canary:pass-canary@example.test/private-path-canary?apikey=query-canary", strings.NewReader("request-body-canary"))
	req.Header.Set("Authorization", "Bearer header-canary")
	done := make(chan error, 1)
	go func() {
		resp, err := (Transport{Base: base, Integration: "source"}).RoundTrip(req)
		if resp != nil {
			resp.Body.Close()
		}
		done <- err
	}()
	<-entered
	if p := r.Read("", 10, "", ""); len(p.Entries) != 1 || p.Entries[0].Phase != "request.started" {
		t.Fatal("slow operation invisible")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	failure := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("upstream-error-canary cookie-canary")
	})
	_, _ = (Transport{Base: failure, Integration: "source"}).RoundTrip(req)
	p := r.Read("", 10, "", "")
	if len(p.Entries) != 4 || *p.Entries[1].Status != 429 || *p.Entries[1].RetryAfter != 3600 || *p.Entries[3].Code != "network-unavailable" {
		t.Fatal("missing individual attempts")
	}
	data, _ := json.Marshal(p)
	files, _ := filepath.Glob(filepath.Join(root, "logs", "*.ndjson"))
	for _, file := range files {
		stored, _ := os.ReadFile(file)
		data = append(data, stored...)
	}
	for _, secret := range []string{"user-canary", "pass-canary", "private-path-canary", "query-canary", "header-canary", "cookie-canary", "body-canary", "upstream-error-canary", "example.test"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("private data retained", secret)
		}
	}
	for _, e := range p.Entries {
		if e.RequestID != RequestID(ctx) {
			t.Fatal("correlation lost")
		}
	}
}
func TestTransportClassifiesCancellationAndDeadline(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		r := New("")
		ctx := WithRecorder(context.Background(), r)
		var cancel context.CancelFunc
		if deadline {
			ctx, cancel = context.WithDeadline(ctx, time.Now().Add(-time.Second))
		} else {
			ctx, cancel = context.WithCancel(ctx)
		}
		cancel()
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.invalid", nil)
		_, _ = (Transport{Integration: "metadata", Base: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, ctx.Err() })}).RoundTrip(req)
		code := "request-cancelled"
		if deadline {
			code = "request-timeout"
		}
		p := r.Read("", 10, "", "")
		if len(p.Entries) != 2 || *p.Entries[1].Code != code {
			t.Fatal("wrong cancellation", p)
		}
	}
}
