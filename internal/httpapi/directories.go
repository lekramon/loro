package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/lekramon/piratarr/internal/settings"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func (h *Handler) directoryAPI(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if h.stopping.Load() {
		fail(w, 503, "not-ready", "The service is stopping.")
		return
	}
	if r.URL.Path == "/api/v1/directories/defaults" {
		if r.Method != "POST" {
			methodNotAllowed(w, "POST")
			return
		}
		body, err := io.ReadAll(r.Body)
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(w, 413, "request-too-large", "Request exceeds 64 KiB.")
			return
		}
		var object map[string]json.RawMessage
		if err != nil || json.Unmarshal(body, &object) != nil || object == nil || len(object) != 0 {
			fail(w, 400, "invalid-request", "Provide one valid JSON object.")
			return
		}
		home, err := os.UserHomeDir()
		if err != nil {
			fail(w, 503, "folders-unavailable", "Folders are unavailable.")
			return
		}
		root := filepath.Join(home, "Videos", "Piratarr")
		if os.Getenv("PIRATARR_CONTAINER") == "1" || os.Getenv("PIRATARR_CONTAINER") == "true" {
			root = "/data"
		}
		if h.defaultRoot != "" {
			root = h.defaultRoot
		}
		dirs := settings.Directories{DownloadDirectory: filepath.Join(root, "downloads"), MovieDirectory: filepath.Join(root, "movies")}
		private, err := filepath.EvalSymlinks(h.storage.DataDirectory())
		if err != nil {
			fail(w, 503, "folders-unavailable", "Folders are unavailable.")
			return
		}
		for _, candidate := range []string{dirs.DownloadDirectory, dirs.MovieDirectory} {
			resolved, e := resolveFutureDirectory(candidate)
			if e != nil || containsDirectory(private, resolved) || containsDirectory(resolved, private) {
				fail(w, 422, "folders-unavailable", "Choose a directory separate from application data.")
				return
			}
		}
		for _, p := range []string{dirs.DownloadDirectory, dirs.MovieDirectory} {
			if err = os.MkdirAll(p, 0755); err != nil {
				fail(w, 422, "folders-unavailable", "Folders are unavailable.")
				return
			}
		}
		if _, err = settings.Validate(r.Context(), settings.Input{Directories: dirs, Profile: settings.Profile{AcceptedResolutions: settings.Resolutions(), PreferredResolution: "1080p", AudioPreferences: []string{"original"}}}, h.storage.DataDirectory()); err != nil {
			settingsFailure(w, err)
			return
		}
		writeJSON(w, 200, dirs)
		return
	}
	if r.Method != "GET" {
		methodNotAllowed(w, "GET")
		return
	}
	requested := r.URL.Query().Get("path")
	if requested == "" {
		requested, _ = os.UserHomeDir()
	}
	p, err := filepath.EvalSymlinks(requested)
	if err != nil || !filepath.IsAbs(p) {
		fail(w, 422, "folders-unavailable", "Folders are unavailable.")
		return
	}
	private, err := filepath.EvalSymlinks(h.storage.DataDirectory())
	if err != nil {
		fail(w, 503, "folders-unavailable", "Folders are unavailable.")
		return
	}
	hidden := func(candidate string) bool { return containsDirectory(private, candidate) }
	if hidden(p) {
		fail(w, 403, "forbidden", "Application data cannot be selected.")
		return
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		fail(w, 422, "folders-unavailable", "Folders are unavailable.")
		return
	}
	directories := []string{}
	for _, entry := range entries {
		if r.Context().Err() != nil {
			fail(w, 503, "folders-unavailable", "Folders are unavailable.")
			return
		}
		candidate := filepath.Join(p, entry.Name())
		resolved, e := filepath.EvalSymlinks(candidate)
		if e != nil || hidden(resolved) {
			continue
		}
		info, e := os.Stat(resolved)
		if e == nil && info.IsDir() {
			directories = append(directories, candidate)
		}
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 || offset > len(directories) {
		offset = 0
	}
	end := offset + 100
	if end > len(directories) {
		end = len(directories)
	}
	next := -1
	if end < len(directories) {
		next = end
	}
	parent := filepath.Dir(p)
	writeJSON(w, 200, struct {
		Path        string   `json:"path"`
		Parent      string   `json:"parent"`
		Directories []string `json:"directories"`
		NextOffset  int      `json:"nextOffset"`
	}{p, parent, directories[offset:end], next})
}

// Resolve existing ancestors before creating defaults, including symlinked parents.
func resolveFutureDirectory(candidate string) (string, error) {
	suffix := []string{}
	current := candidate
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func containsDirectory(base, candidate string) bool {
	if runtime.GOOS == "windows" {
		base = strings.ToLower(base)
		candidate = strings.ToLower(candidate)
	}
	rel, err := filepath.Rel(base, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
