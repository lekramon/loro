// Package browser manages one ephemeral local browser operation at a time.
package browser

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"github.com/lekramon/piratarr/internal/communications"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

//go:embed helper.py
var helper string

type Row struct {
	Title, Path, Category, Size, Seeders string
	Position                             int
}
type Result struct {
	Error    string `json:"error"`
	Rows     []Row  `json:"rows"`
	RawCount int    `json:"rawCount"`
	HasMore  bool   `json:"hasMore"`
	Magnet   string `json:"magnet"`
}

var gate = make(chan struct{}, 1)

func Fetch(ctx context.Context, address, mode string) (result Result) {
	ctx = communications.Correlate(ctx)
	operation := "browser.search"
	if mode == "detail" {
		operation = "browser.detail"
	}
	communications.Emit(ctx, communications.Event{Integration: "browser", Operation: operation, Phase: "operation.started"})
	start := time.Now()
	defer func() {
		event := communications.Event{Integration: "browser", Operation: operation, Phase: "operation.completed", ElapsedMS: communications.Ptr(time.Since(start).Milliseconds())}
		if result.Error != "" {
			event.Phase = "operation.failed"
			event.Code = &result.Error
			event.Level = "warning"
		}
		communications.Emit(ctx, event)
		communications.Emit(ctx, communications.Event{Integration: "browser", Operation: operation, Phase: "browser.stopped"})
	}()
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	select {
	case gate <- struct{}{}:
		defer func() { <-gate }()
	case <-ctx.Done():
		return Result{Error: "source-timeout"}
	}
	executable := os.Getenv("LOURO_BROWSER_PYTHON")
	if executable == "" {
		if runtime.GOOS == "windows" {
			executable = filepath.Join(os.Getenv("LOCALAPPDATA"), "LouroBrowser", "Scripts", "python.exe")
		} else {
			executable = "/opt/louro-browser/bin/python"
		}
	}
	payload, _ := json.Marshal(map[string]string{"url": address, "mode": mode})
	cmd := exec.CommandContext(ctx, executable, "-c", helper)
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Stderr = &traceWriter{ctx: ctx, operation: operation}
	cmd.WaitDelay = time.Second
	output := &boundedOutput{}
	cmd.Stdout = output
	cleanup, err := prepareProcess(cmd)
	if err != nil {
		return Result{Error: "source-browser-unavailable"}
	}
	defer cleanup()
	if err = cmd.Start(); err != nil {
		if ctx.Err() != nil {
			return Result{Error: "source-timeout"}
		}
		return Result{Error: "source-browser-unavailable"}
	}
	if err = attachProcess(cmd); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if ctx.Err() != nil {
			return Result{Error: "source-timeout"}
		}
		return Result{Error: "source-browser-unavailable"}
	}
	err = cmd.Wait()
	if ctx.Err() != nil {
		return Result{Error: "source-timeout"}
	}
	if err != nil {
		return Result{Error: "source-browser-unavailable"}
	}
	if json.Unmarshal(output.Bytes(), &result) != nil || result.RawCount < 0 || result.RawCount > 40 || len(result.Rows) > 40 {
		return Result{Error: "source-invalid-response"}
	}
	switch result.Error {
	case "", "source-browser-unavailable", "source-unavailable", "source-timeout", "source-challenge-required", "source-invalid-response", "source-rate-limited", "missing-reference", "ambiguous-reference", "unsupported-reference":
	default:
		result = Result{Error: "source-invalid-response"}
	}
	return result
}

type boundedOutput struct{ bytes.Buffer }

func (b *boundedOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 2<<20 {
		return 0, errors.New("browser output exceeds limit")
	}
	return b.Buffer.Write(p)
}

func ValidDetail(path string) bool {
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[0] != "" || parts[1] != "torrent" || parts[2] == "" {
		return false
	}
	for _, c := range parts[2] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return !strings.ContainsAny(path, "?#\\") && !strings.Contains(path, "..")
}
