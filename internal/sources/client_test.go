package sources

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func TestTimedOutSearchIsNotReplayed(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		<-r.Context().Done()
	}))
	defer server.Close()
	client := NewClient()
	client.searchHTTP.Timeout = 30 * time.Millisecond
	_, err := client.request(context.Background(), Config{Public: Public{URL: server.URL}}, url.Values{"t": {"movie"}})
	var provider *ProviderError
	if !errors.As(err, &provider) || provider.Code != "source-timeout" || calls.Load() != 1 {
		t.Fatal("timeout replayed or misclassified", calls.Load(), err)
	}
}
