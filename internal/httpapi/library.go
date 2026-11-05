package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/lekramon/piratarr/internal/library"
	"github.com/lekramon/piratarr/internal/settings"
)

func (h *Handler) libraryAPI(w http.ResponseWriter, r *http.Request) {
	scan := r.URL.Path == "/api/v1/library/scan"
	method := "GET"
	if scan {
		method = "POST"
	}
	if r.Method != method {
		methodNotAllowed(w, method)
		return
	}
	if h.stopping.Load() || h.library == nil {
		fail(w, 503, "not-ready", "Library inventory is unavailable.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if h.storage.Ready(ctx) != nil {
		fail(w, 503, "storage-unavailable", "Local inventory storage is unavailable.")
		return
	}
	var result library.Snapshot
	var err error
	if scan {
		data, readErr := io.ReadAll(r.Body)
		var oversized *http.MaxBytesError
		if errors.As(readErr, &oversized) {
			fail(w, 413, "request-too-large", "Request exceeds 64 KiB.")
			return
		}
		var body map[string]json.RawMessage
		if readErr != nil || json.Unmarshal(data, &body) != nil || body == nil || len(body) != 0 {
			fail(w, 400, "invalid-request", "Provide an empty JSON object.")
			return
		}
		if _, ok := catalogQuery(w, r); !ok {
			return
		}
		result, err = h.library.Scan(ctx)
	} else {
		values, ok := catalogQuery(w, r, "limit", "offset", "status")
		if !ok {
			return
		}
		limit, validLimit := boundedInteger(values, "limit", 50, 1, 100)
		offset, validOffset := boundedInteger(values, "offset", 0, 0, 1000000)
		status := values.Get("status")
		if status == "" {
			status = "all"
		}
		if !validLimit || !validOffset || (status != "all" && status != "present" && status != "missing") {
			fail(w, 422, "validation-failed", "Provide valid pagination and inventory status.")
			return
		}
		result, err = h.library.List(ctx, limit, offset, status)
	}
	if err != nil {
		switch {
		case errors.Is(err, settings.ErrSetupRequired):
			fail(w, 409, "setup-required", "Complete setup before scanning movie files.")
		case errors.Is(err, library.ErrBusy):
			fail(w, 409, "scan-in-progress", "A library scan is already running.")
		case errors.Is(err, library.ErrChanged):
			fail(w, 409, "settings-changed", "Settings changed during the scan. Scan the current folder again.")
		case errors.Is(err, library.ErrLimit):
			fail(w, 422, "scan-limit", "The scan exceeded its supported file, entry or depth limit.")
		default:
			fail(w, 503, "scan-unavailable", "Unable to complete library inventory. Refresh saved inventory before retrying.")
		}
		return
	}
	writeJSON(w, 200, result)
}
