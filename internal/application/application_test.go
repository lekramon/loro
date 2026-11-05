package application

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lekramon/piratarr/internal/bootstrap"
)

func TestSubprocess(t *testing.T) {
	if os.Getenv("PIRATARR_TEST_CHILD") != "1" {
		return
	}
	app, err := Open(context.Background(), bootstrap.Config{DataDir: os.Getenv("PIRATARR_TEST_DATA"), Listen: "127.0.0.1:0", Workers: 4}, fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("Piratarr")}})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(21)
	}
	go func() { _ = app.Serve() }()
	fmt.Println(app.Address())
	select {}
}

func childCommand(ctx context.Context, directory string) *exec.Cmd {
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSubprocess$")
	command.Env = append(os.Environ(), "PIRATARR_TEST_CHILD=1", "PIRATARR_TEST_DATA="+directory)
	return command
}

func TestInstanceExclusionAndProcessDeathRecovery(t *testing.T) {
	directory := t.TempDir()
	start := func() (*exec.Cmd, string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		t.Cleanup(cancel)
		child := childCommand(ctx, directory)
		output, err := child.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = child.Process.Kill() })
		address := make(chan string, 1)
		go func() {
			scanner := bufio.NewScanner(output)
			if scanner.Scan() {
				address <- scanner.Text()
			} else {
				address <- ""
			}
		}()
		select {
		case value := <-address:
			if value == "" {
				t.Fatal("child exited before readiness")
			}
			return child, value
		case <-ctx.Done():
			t.Fatal("child startup timed out")
		}
		return nil, ""
	}
	first, address := start()
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://" + address + "/api/v1/health/ready")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("first process not ready: %d", response.StatusCode)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := childCommand(ctx, directory).CombinedOutput()
	if err == nil || !strings.Contains(string(output), "another instance") {
		t.Fatalf("second process was not excluded: %v %s", err, output)
	}
	if err := first.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = first.Wait()
	recovered, recoveredAddress := start()
	response, err = client.Get("http://" + recoveredAddress + "/api/v1/health/ready")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("restart after process death not ready")
	}
	_ = recovered.Process.Kill()
	_ = recovered.Wait()
}

func TestStartupFailureAndGracefulShutdown(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), bootstrap.Config{DataDir: file, Listen: "127.0.0.1:0"}, fstest.MapFS{}); err == nil {
		t.Fatal("started with invalid storage")
	}
	directory := t.TempDir()
	app, err := Open(context.Background(), bootstrap.Config{DataDir: directory, Listen: "127.0.0.1:0"}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- app.Serve() }()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := app.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != http.ErrServerClosed {
		t.Fatalf("unexpected shutdown: %v", err)
	}
	again, err := Open(context.Background(), bootstrap.Config{DataDir: directory, Listen: "127.0.0.1:0"}, fstest.MapFS{})
	if err != nil {
		t.Fatalf("shutdown did not release storage: %v", err)
	}
	if err := again.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestServiceCredentialSurvivesApplicationRestart(t *testing.T) {
	directory := t.TempDir()
	for pass := 0; pass < 2; pass++ {
		app, err := Open(context.Background(), bootstrap.Config{DataDir: directory, Listen: "127.0.0.1:0", Workers: 4, TMDBTokenFile: filepath.Join(t.TempDir(), "missing")}, fstest.MapFS{})
		if err != nil {
			t.Fatal(err)
		}
		func() {
			defer app.Shutdown(context.Background())
			if pass == 0 {
				req := httptest.NewRequest("PUT", "http://"+app.Address()+"/api/v1/metadata/credential", strings.NewReader(`{"token":"dummy-restart"}`))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Piratarr-Request", "1")
				result := httptest.NewRecorder()
				app.Handler.ServeHTTP(result, req)
				if result.Code != 200 {
					t.Fatal(result.Code, result.Body.String())
				}
			}
			req := httptest.NewRequest("GET", "http://"+app.Address()+"/api/v1/metadata/status", nil)
			result := httptest.NewRecorder()
			app.Handler.ServeHTTP(result, req)
			if result.Code != 200 || !strings.Contains(result.Body.String(), `"configured":true`) || strings.Contains(result.Body.String(), "dummy-restart") {
				t.Fatal(result.Code, result.Body.String())
			}
			info, err := os.Stat(filepath.Join(directory, "credentials", "tmdb-read-token"))
			if err != nil {
				t.Fatal(err)
			}
			if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
				t.Fatal("credential permissions", info.Mode())
			}
		}()
	}
}
