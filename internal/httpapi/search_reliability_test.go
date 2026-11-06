package httpapi

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReleaseSearchSlowSuccessWithoutRepeatedRequests(t *testing.T) {
	f := newReleaseFixture(t)
	f.block.Store(true)
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- f.request(t, "GET", f.path(""), "") }()
	<-f.queries
	time.Sleep(2300 * time.Millisecond)
	close(f.release)
	response := <-done
	if response.Code != 200 || f.calls.Load() != 1 {
		t.Fatalf("slow valid feed: status=%d calls=%d", response.Code, f.calls.Load())
	}
}

func TestReleaseSearchCancellationReleasesExclusion(t *testing.T) {
	f := newReleaseFixture(t)
	f.block.Store(true)
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest("GET", f.path(""), nil).WithContext(ctx)
	request.Host = "127.0.0.1:8080"
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { f.handler.ServeHTTP(response, request); close(done) }()
	select {
	case <-f.queries:
	case <-time.After(time.Second):
		t.Fatal("search not started")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancellation did not stop search")
	}
	if f.calls.Load() != 1 || !strings.Contains(response.Body.String(), "source-timeout") {
		t.Fatal(response.Code, response.Body.String())
	}
	f.block.Store(false)
	if r := f.request(t, "GET", f.path(""), ""); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
}
