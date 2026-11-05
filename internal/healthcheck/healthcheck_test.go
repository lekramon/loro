package healthcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLocalReadinessContract(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		success bool
	}{{"ready", 200, `{"status":"ready"}`, true}, {"not ready", 503, `{"status":"ready"}`, false}, {"wrong body", 200, `{"status":"alive"}`, false}, {"not JSON", 200, "<html>OK</html>", false}, {"trailing", 200, `{"status":"ready"}{}`, false}, {"redirect", 302, `{"status":"ready"}`, false}} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/health/ready" || r.Method != "GET" {
					t.Error("wrong readiness request")
				}
				w.Header().Set("Location", "http://example.invalid")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
			t.Setenv("NO_PROXY", "")
			err := Run(context.Background(), []string{"--listen", strings.TrimPrefix(server.URL, "http://")}, func(string) string { return "invalid environment overridden by flag" })
			if (err == nil) != tc.success {
				t.Fatalf("success=%v err=%v", tc.success, err)
			}
		})
	}
}

func TestLouroListenerOverridesLegacyHealthcheckEnvironment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"status":"ready"}`)) }))
	defer server.Close()
	getenv := func(key string) string {
		if key == "LOURO_LISTEN" {
			return strings.TrimPrefix(server.URL, "http://")
		}
		if key == "PIRATARR_LISTEN" {
			return "127.0.0.1:1"
		}
		return ""
	}
	if err := Run(context.Background(), nil, getenv); err != nil {
		t.Fatal("healthcheck did not use Louro listener", err)
	}
}
func TestInvalidTargetsAndTimeout(t *testing.T) {
	for _, target := range []string{"example.com:8080", "192.0.2.1:8080", "127.0.0.1:0", "127.0.0.1:65536", "http://localhost:8080", ""} {
		if Run(context.Background(), []string{"--listen", target}, func(string) string { return "" }) == nil {
			t.Errorf("accepted target %q", target)
		}
	}
	for _, args := range [][]string{{"--unknown"}, {"extra"}} {
		if Run(context.Background(), args, func(string) string { return "" }) == nil {
			t.Error("accepted invalid options")
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	start := time.Now()
	if Run(ctx, nil, func(string) string { return strings.TrimPrefix(server.URL, "http://") }) == nil {
		t.Fatal("timed out request succeeded")
	}
	if time.Since(start) > time.Second {
		t.Fatal("context deadline was not honored")
	}
}
