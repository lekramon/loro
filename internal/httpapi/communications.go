package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lekramon/piratarr/internal/communications"
)

func (h *Handler) communicationsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		methodNotAllowed(w, "GET")
		return
	}
	if h.stopping.Load() || h.communications == nil {
		fail(w, 503, "not-ready", "Communication history is unavailable.")
		return
	}
	q, ok := catalogQuery(w, r, "after", "limit", "integration", "requestId")
	if !ok {
		return
	}
	if r.ContentLength != 0 || len(r.TransferEncoding) > 0 {
		fail(w, 400, "invalid-request", "Communication history accepts no body.")
		return
	}
	limit, valid := boundedInteger(q, "limit", 100, 1, 200)
	if !valid || q.Has("integration") && !communications.ValidIntegration(q.Get("integration")) || q.Has("requestId") && !communications.ValidRequestID(q.Get("requestId")) {
		fail(w, 422, "validation-failed", "Choose a valid diagnostics filter and limit 1–200.")
		return
	}
	if q.Has("after") {
		if _, _, err := communications.ParseCursor(q.Get("after")); err != nil {
			fail(w, 422, "validation-failed", "Provide a returned communication cursor.")
			return
		}
	}
	writeJSON(w, 200, h.communications.Read(q.Get("after"), limit, q.Get("integration"), q.Get("requestId")))
}

func (h *Handler) BackgroundContext(ctx context.Context) context.Context {
	return communications.WithRecorder(ctx, h.communications)
}

type communicationResponse struct {
	http.ResponseWriter
	status int
	code   string
}

func (w *communicationResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *communicationResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *communicationResponse) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(data)
}
func (w *communicationResponse) diagnosticCode(code string) { w.code = code }

func apiOperation(path string) string {
	for _, value := range []struct{ prefix, name string }{{"/api/v1/source/test", "source.test"}, {"/api/v1/source", "source.configuration"}, {"/api/v1/releases", "source.search"}, {"/api/v1/metadata", "metadata.request"}, {"/api/v1/catalog", "catalog.request"}, {"/api/v1/download-client", "download-client.request"}, {"/api/v1/acquisitions", "acquisition.request"}, {"/api/v1/downloads", "downloads.request"}, {"/api/v1/library", "library.request"}, {"/api/v1/setup", "setup.request"}, {"/api/v1/settings", "settings.request"}, {"/api/v1/directories", "directories.request"}} {
		if strings.HasPrefix(path, value.prefix) {
			return "api." + value.name
		}
	}
	return "api.request"
}
func (h *Handler) recordAPI(w http.ResponseWriter, r *http.Request) {
	ctx := communications.Correlate(h.BackgroundContext(r.Context()))
	r = r.WithContext(ctx)
	w.Header().Set("X-Louro-Request-Id", communications.RequestID(ctx))
	operation := apiOperation(r.URL.Path)
	start := time.Now()
	communications.Emit(ctx, communications.Event{Integration: "api", Operation: operation, Phase: "request.started"})
	response := &communicationResponse{ResponseWriter: w}
	defer func() {
		problem := recover()
		status := response.status
		if status == 0 {
			status = 200
		}
		if problem != nil {
			status = 500
			response.code = "internal-error"
		}
		e := communications.Event{Integration: "api", Operation: operation, Phase: "response.received", Status: communications.Ptr(status), ElapsedMS: communications.Ptr(time.Since(start).Milliseconds())}
		if status >= 400 {
			e.Level = "warning"
		}
		if response.code != "" {
			e.Code = &response.code
		}
		if delay, err := strconv.Atoi(response.Header().Get("Retry-After")); err == nil && delay >= 0 && delay <= 3600 {
			e.RetryAfter = &delay
		}
		communications.Emit(ctx, e)
		if problem != nil {
			panic(problem)
		}
	}()
	h.api(response, r)
}
