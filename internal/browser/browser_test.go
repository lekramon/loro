package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lekramon/piratarr/internal/communications"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("LOURO_BROWSER_TEST_HELPER") == "child" {
		time.Sleep(time.Hour)
		os.Exit(0)
	}
	if os.Getenv("LOURO_BROWSER_TEST_HELPER") == "parent" {
		child := exec.Command(os.Args[0])
		child.Env = append(os.Environ(), "LOURO_BROWSER_TEST_HELPER=child")
		if child.Start() != nil {
			os.Exit(2)
		}
		_ = os.WriteFile(os.Getenv("LOURO_BROWSER_TEST_PID"), []byte(strconv.Itoa(child.Process.Pid)), 0600)
		time.Sleep(time.Hour)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestBrowserMissingAndInvalidDetail(t *testing.T) {
	t.Setenv("LOURO_BROWSER_PYTHON", filepath.Join(t.TempDir(), "absent"))
	if result := Fetch(context.Background(), "http://localhost/", "search"); result.Error != "source-browser-unavailable" {
		t.Fatal(result)
	}
	for _, path := range []string{"/torrent/1/title/", "/torrent/999/a-b/"} {
		if !ValidDetail(path) {
			t.Fatal(path)
		}
	}
	for _, path := range []string{"//other/torrent/1/", "/torrent/no/a/", "/torrent/1/../x", "/torrent/1/a?q=1"} {
		if ValidDetail(path) {
			t.Fatal(path)
		}
	}
}

func TestBrowserCancellationKillsChildTree(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	t.Setenv("LOURO_BROWSER_PYTHON", os.Args[0])
	t.Setenv("LOURO_BROWSER_TEST_HELPER", "parent")
	t.Setenv("LOURO_BROWSER_TEST_PID", pidFile)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan Result, 1)
	go func() { done <- Fetch(ctx, "http://localhost/", "search") }()
	var data []byte
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		data, _ = os.ReadFile(pidFile)
		if len(data) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(data) == 0 {
		cancel()
		<-done
		t.Fatal("helper child did not start")
	}
	pid, _ := strconv.Atoi(string(data))
	cancel()
	select {
	case result := <-done:
		if result.Error != "source-timeout" {
			t.Fatal(result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("helper did not stop")
	}
	until = time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		if !processAlive(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("browser child survived cancellation")
}

func TestInstalledBrowserWithLocalFixture(t *testing.T) {
	if os.Getenv("LOURO_TEST_BROWSER") != "1" {
		t.Skip("explicit installed-browser check")
	}
	var escaped atomic.Int32
	outside := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { escaped.Add(1) }))
	defer outside.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect/1/" {
			http.Redirect(w, r, outside.URL, http.StatusFound)
			return
		}
		if r.URL.Path == "/challenge/1/" {
			w.Header().Set("Cf-Mitigated", "challenge")
			fmt.Fprint(w, `<html><title>Just a moment</title><body>Cloudflare<script>setTimeout(()=>location.href='/search/Example/1/',300)</script></body></html>`)
			return
		}
		fmt.Fprint(w, `<html><head><title>Movie results</title></head>`)
		w.(http.Flusher).Flush()
		time.Sleep(time.Second) // Simulate a streaming document after challenge navigation.
		fmt.Fprint(w, `<html><body><table class="table-list"><tr><td class="coll-1"><a href="/sub/42/">HD</a><a href="/torrent/123/Example/">Example.2024.1080p.mkv ⭐</a></td><td class="coll-2">7</td><td class="coll-4">1.5 GB</td></tr></table></body></html>`)
	}))
	defer server.Close()
	result := Fetch(context.Background(), server.URL+"/search/Example/1/", "search")
	if result.Error != "" || len(result.Rows) != 1 || result.Rows[0].Title != "Example.2024.1080p.mkv ⭐" {
		data, _ := json.Marshal(result)
		t.Fatal(string(data))
	}
	recorder := communications.New(t.TempDir())
	result = Fetch(communications.WithRecorder(context.Background(), recorder), server.URL+"/challenge/1/", "search")
	if result.Error != "" || len(result.Rows) != 1 {
		t.Fatal("did not resume after simulated challenge", result.Error)
	}
	phases := map[string]int{}
	for _, event := range recorder.Read("", 200, "browser", "").Entries {
		phases[event.Phase]++
	}
	for _, phase := range []string{"operation.started", "browser.started", "browser.navigation", "browser.response", "browser.challenge.detected", "browser.challenge.cleared", "browser.parsed", "operation.completed", "browser.stopped"} {
		if phases[phase] == 0 {
			t.Fatal("missing browser evidence", phase, phases)
		}
	}
	result = Fetch(context.Background(), server.URL+"/redirect/1/", "search")
	if result.Error != "source-invalid-response" {
		t.Fatal("accepted cross-origin results", result.Error, escaped.Load())
	}
}

func TestInstalledBrowserPersistentChallengeStaysUnresolved(t *testing.T) {
	if os.Getenv("LOURO_TEST_BROWSER") != "1" {
		t.Skip("explicit installed-browser check")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cf-Mitigated", "challenge")
		w.WriteHeader(403)
		fmt.Fprint(w, `<html><title>Just a moment</title><body>Cloudflare<div style="width:300px;height:70px"><input type="hidden" name="cf-turnstile-response"></div></body></html>`)
	}))
	defer server.Close()
	recorder := communications.New(t.TempDir())
	result := Fetch(communications.WithRecorder(context.Background(), recorder), server.URL+"/challenge/1/", "search")
	if result.Error != "source-challenge-required" {
		t.Fatal("persistent challenge misclassified", result.Error)
	}
	phases := map[string]int{}
	for _, event := range recorder.Read("", 200, "browser", "").Entries {
		phases[event.Phase]++
	}
	if phases["browser.challenge.detected"] == 0 || phases["browser.interaction"] == 0 || phases["browser.stopped"] != 1 || phases["browser.challenge.cleared"] != 0 || phases["operation.completed"] != 0 || phases["browser.parsed"] != 0 {
		t.Fatal("interaction incorrectly proved clearance", phases)
	}
}
