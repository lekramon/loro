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
)

type Storage interface {
	Ready(context.Context) error
	SetupCompleted(context.Context) (bool, error)
}

type Handler struct {
	storage  Storage
	assets   fs.FS
	port     string
	stopping atomic.Bool
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func New(storage Storage, assets fs.FS, listen string) *Handler {
	_, port, _ := net.SplitHostPort(listen)
	return &Handler{storage: storage, assets: assets, port: port}
}

func (h *Handler) Stop() { h.stopping.Store(true) }

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func fail(w http.ResponseWriter, status int, code, message string) {
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
	w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'self'")
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
		h.api(w, r)
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
	switch r.URL.Path {
	case "/api/v1/health/live", "/api/v1/health/ready", "/api/v1/setup", "/api/v1/settings":
		if r.Method != http.MethodGet {
			methodNotAllowed(w, "GET")
			return
		}
	default:
		fail(w, 404, "not-found", "API resource does not exist.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	switch r.URL.Path {
	case "/api/v1/health/live":
		writeJSON(w, 200, map[string]string{"status": "alive"})
	case "/api/v1/health/ready":
		if h.stopping.Load() || h.storage.Ready(ctx) != nil {
			fail(w, 503, "not-ready", "Local storage is unavailable or the service is stopping.")
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ready"})
	case "/api/v1/setup", "/api/v1/settings":
		completed, err := h.storage.SetupCompleted(ctx)
		if err != nil {
			fail(w, 503, "storage-unavailable", "Local settings are unavailable.")
			return
		}
		if r.URL.Path == "/api/v1/settings" {
			if !completed {
				fail(w, 409, "setup-required", "Complete first-run setup before reading settings.")
				return
			}
			fail(w, 503, "storage-unavailable", "Settings reading is unavailable in the startup preview.")
			return
		}
		writeJSON(w, 200, struct {
			Completed   bool       `json:"completed"`
			Resolutions []string   `json:"supportedResolutions"`
			Languages   []language `json:"languages"`
		}{completed, []string{"480p", "576p", "720p", "1080p", "2160p"}, languages})
	}
}

func (h *Handler) frontend(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if filename == "" || filename == "." || filename == "setup" || filename == "settings" {
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
