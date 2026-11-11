package httpapi

import (
	"context"
	"encoding/json"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lekramon/piratarr/internal/acquisitions"
	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/communications"
	"github.com/lekramon/piratarr/internal/downloadclient"
	"github.com/lekramon/piratarr/internal/downloads"
	"github.com/lekramon/piratarr/internal/imports"
	"github.com/lekramon/piratarr/internal/library"
	"github.com/lekramon/piratarr/internal/policies"
	"github.com/lekramon/piratarr/internal/series"
	"github.com/lekramon/piratarr/internal/settings"
	"github.com/lekramon/piratarr/internal/sources"
	"github.com/lekramon/piratarr/internal/submissions"
)

type Storage interface {
	Ready(context.Context) error
	SetupCompleted(context.Context) (bool, error)
	CreateSettings(context.Context, settings.Input) (settings.Settings, error)
	GetSettings(context.Context) (settings.Settings, error)
	UpdateSettings(context.Context, int, settings.Input) (settings.Settings, error)
	policies.Repository
	imports.Repository
	series.Repository
	DataDirectory() string
}

type Handler struct {
	communications *communications.Recorder
	downloadRepo   downloads.Repository
	submissionRepo submissions.Repository
	acquisitions   *acquisitions.Service
	downloadClient *downloadclient.Service
	catalog        *catalog.Service
	library        *library.Service
	source         *sources.Service
	defaultRoot    string
	storage        Storage
	assets         fs.FS
	port           string
	stopping       atomic.Bool
	settings       settings.Service
	importService  *imports.Service
	seriesRepo     series.Repository
}

type apiError struct {
	Code        string                `json:"code"`
	Message     string                `json:"message"`
	FieldErrors []settings.FieldError `json:"fieldErrors,omitempty"`
}

func New(storage Storage, assets fs.FS, listen string, services ...*catalog.Service) *Handler {
	_, port, _ := net.SplitHostPort(listen)
	handler := &Handler{communications: communications.New(storage.DataDirectory()), storage: storage, assets: assets, port: port, settings: settings.Service{Repository: storage, DataDirectory: storage.DataDirectory()}}
	if len(services) > 0 {
		handler.catalog = services[0]
	}
	if repo, ok := storage.(library.Repository); ok {
		handler.library = library.New(repo, storage.DataDirectory())
	}
	if repo, ok := storage.(sources.Repository); ok {
		handler.source = sources.New(repo)
	}
	if repo, ok := storage.(downloadclient.Repository); ok {
		handler.downloadClient = downloadclient.New(repo)
	}
	if repo, ok := storage.(acquisitions.Repository); ok {
		handler.acquisitions = acquisitions.New(repo, handler.source)
	}
	if repo, ok := storage.(submissions.Repository); ok {
		handler.submissionRepo = repo
	}
	if repo, ok := storage.(downloads.Repository); ok {
		handler.downloadRepo = repo
	}
	if repo, ok := storage.(imports.Repository); ok {
		handler.importService = &imports.Service{Repo: repo, Prober: library.NewProber("ffprobe"), Now: time.Now}
	}
	if repo, ok := storage.(series.Repository); ok {
		handler.seriesRepo = repo
	}
	return handler
}

func (h *Handler) Stop() { h.stopping.Store(true) }

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func fail(w http.ResponseWriter, status int, code, message string) {
	if recorder, ok := w.(interface{ diagnosticCode(string) }); ok {
		recorder.diagnosticCode(code)
	}
	writeJSON(w, status, apiError{Code: code, Message: message})
}

func (h *Handler) permittedHost(authority string) bool {
	host, port, err := net.SplitHostPort(authority)
	if err != nil {
		return false
	}
	return port == h.port && (host == "localhost" || host == "127.0.0.1" || host == "::1")
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; frame-ancestors 'none'; object-src 'none'; base-uri 'self'")
	if !h.permittedHost(r.Host) {
		fail(w, 403, "forbidden", "Only the configured loopback host and port are allowed.")
		return
	}
	if origins, exists := r.Header["Origin"]; exists {
		if len(origins) != 1 {
			fail(w, 403, "forbidden", "Origin is not allowed.")
			return
		}
		origin, err := url.Parse(origins[0])
		if err != nil || origin.Scheme != "http" || origin.Host != r.Host || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
			fail(w, 403, "forbidden", "Origin is not allowed.")
			return
		}
	}
	if r.Method == http.MethodOptions {
		fail(w, 403, "forbidden", "Cross-origin preflights are not supported.")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		if r.Header.Get("X-Piratarr-Request") != "1" {
			fail(w, 403, "forbidden", "The mutation request header is required.")
			return
		}
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			fail(w, 415, "unsupported-media-type", "JSON content type is required.")
			return
		}
		if r.ContentLength > 65536 {
			fail(w, 413, "request-too-large", "Request exceeds 64 KiB.")
			return
		}
		// Future mutation handlers must decode through this bounded body.
		r.Body = http.MaxBytesReader(w, r.Body, 65536)
	}
	if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
		if r.URL.Path == "/api/v1/system/communications" || strings.HasPrefix(r.URL.Path, "/api/v1/health/") || strings.HasPrefix(r.URL.Path, "/api/v1/series") || strings.HasPrefix(r.URL.Path, "/api/v1/episodes/") {
			h.api(w, r)
		} else {
			h.recordAPI(w, r)
		}
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w, "GET, HEAD")
		return
	}
	h.frontend(w, r)
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	fail(w, 405, "method-not-allowed", "Method is not supported by this resource.")
}

func (h *Handler) api(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/v1/episodes/search" {
		h.episodeSearch(w, r)
		return
	}
	if h.seriesAPI(w, r) {
		return
	}
	if r.URL.Path == "/api/v1/system/communications" {
		h.communicationsAPI(w, r)
		return
	}
	if r.URL.Path == "/api/v1/system/operations" {
		h.operationsAPI(w, r)
		return
	}
	if r.URL.Path == "/api/v1/downloads" || strings.HasPrefix(r.URL.Path, "/api/v1/downloads/") {
		h.downloadAPI(w, r)
		return
	}
	if r.URL.Path == "/api/v1/imports" || r.URL.Path == "/api/v1/imports/preview" {
		h.importAPI(w, r)
		return
	}
	if r.URL.Path == "/api/v1/acquisitions" || strings.HasPrefix(r.URL.Path, "/api/v1/acquisitions/") {
		h.acquisitionAPI(w, r)
		return
	}
	if r.URL.Path == "/api/v1/download-client" || r.URL.Path == "/api/v1/download-client/test" {
		h.downloadClientAPI(w, r)
		return
	}
	if r.URL.Path == "/api/v1/releases" {
		h.releaseSearch(w, r)
		return
	}
	if r.URL.Path == "/api/v1/source" || r.URL.Path == "/api/v1/source/test" {
		h.sourceAPI(w, r)
		return
	}
	if r.URL.Path == "/api/v1/sources" || strings.HasPrefix(r.URL.Path, "/api/v1/sources/") {
		h.sourcesAPI(w, r)
		return
	}
	if r.URL.Path == "/api/v1/acquisition-profiles" || strings.HasPrefix(r.URL.Path, "/api/v1/acquisition-profiles/") || strings.HasPrefix(r.URL.Path, "/api/v1/quality-definitions") || strings.HasPrefix(r.URL.Path, "/api/v1/custom-formats") || strings.HasPrefix(r.URL.Path, "/api/v1/catalog/movies/") && strings.HasSuffix(r.URL.Path, "/acquisition-profile") {
		h.policyAPI(w, r)
		return
	}
	if r.URL.Path == "/api/v1/metadata/credential" {
		h.metadataCredential(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/library/files/") {
		h.reviewAPI(w, r)
		return
	}
	if r.URL.Path == "/api/v1/library/scan" || r.URL.Path == "/api/v1/library/files" {
		h.libraryAPI(w, r)
		return
	}
	if h.catalogAPI(w, r) {
		return
	}
	if r.URL.Path == "/api/v1/directories" || r.URL.Path == "/api/v1/directories/defaults" {
		h.directoryAPI(w, r)
		return
	}
	switch r.URL.Path {
	case "/api/v1/setup":
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			methodNotAllowed(w, "GET, POST")
			return
		}
	case "/api/v1/settings":
		if r.Method != http.MethodGet && r.Method != http.MethodPut {
			methodNotAllowed(w, "GET, PUT")
			return
		}
	case "/api/v1/health/live", "/api/v1/health/ready":
		if r.Method != http.MethodGet {
			methodNotAllowed(w, "GET")
			return
		}
	default:
		fail(w, 404, "not-found", "API resource does not exist.")
		return
	}
	timeout := 2 * time.Second
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	if r.Method == http.MethodPut && r.URL.Path == "/api/v1/settings" {
		if h.stopping.Load() {
			fail(w, 503, "not-ready", "The service is stopping.")
			return
		}
		input, ok := decodeSettingsBody[settings.UpdateInput](w, r, true)
		if !ok {
			return
		}
		result, err := h.settings.Update(ctx, input)
		if err != nil {
			settingsFailure(w, err)
			return
		}
		writeJSON(w, 200, result)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/api/v1/setup" {
		if h.stopping.Load() {
			fail(w, 503, "not-ready", "The service is stopping.")
			return
		}
		input, ok := decodeSetup(w, r)
		if !ok {
			return
		}
		result, err := h.settings.CompleteSetup(ctx, input)
		if err != nil {
			settingsFailure(w, err)
			return
		}
		writeJSON(w, 201, result)
		return
	}
	switch r.URL.Path {
	case "/api/v1/health/live":
		writeJSON(w, 200, map[string]string{"status": "alive"})
	case "/api/v1/health/ready":
		if h.stopping.Load() || h.storage.Ready(ctx) != nil {
			fail(w, 503, "not-ready", "Local storage is unavailable or the service is stopping.")
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ready"})
	case "/api/v1/settings":
		result, err := h.storage.GetSettings(ctx)
		if err != nil {
			settingsFailure(w, err)
			return
		}
		writeJSON(w, 200, result)
	case "/api/v1/setup":
		completed, err := h.storage.SetupCompleted(ctx)
		if err != nil {
			fail(w, 503, "storage-unavailable", "Local settings are unavailable.")
			return
		}
		writeJSON(w, 200, struct {
			Completed   bool       `json:"completed"`
			Resolutions []string   `json:"supportedResolutions"`
			Languages   []language `json:"languages"`
		}{completed, settings.Resolutions(), languages})
	}
}

func (h *Handler) frontend(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if filename == "" || filename == "." || filename == "setup" || filename == "settings" || strings.HasPrefix(filename, "settings/") || filename == "preview" || filename == "about" || filename == "search" {
		filename = "index.html"
	}
	file, err := fs.ReadFile(h.assets, filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType := mime.TypeByExtension(path.Ext(filename))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	if strings.HasPrefix(filename, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	if r.Method != http.MethodHead {
		_, _ = w.Write(file)
	}
}
