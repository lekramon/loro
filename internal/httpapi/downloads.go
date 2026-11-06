package httpapi

import (
	"context"
	"errors"
	"github.com/lekramon/piratarr/internal/acquisitions"
	"github.com/lekramon/piratarr/internal/downloads"
	"net/http"
	"strings"
	"time"
)

func (h *Handler) downloadAPI(w http.ResponseWriter, r *http.Request) {
	root := r.URL.Path == "/api/v1/downloads"
	id, action := "", ""
	if !root {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/downloads/"), "/")
		if len(parts) < 1 || len(parts) > 2 || !acquisitions.ValidID(parts[0]) {
			downloadFailure(w, downloads.ErrNotFound)
			return
		}
		id = parts[0]
		if len(parts) == 2 {
			action = parts[1]
			if action != "check" {
				fail(w, 404, "not-found", "Resource does not exist.")
				return
			}
		}
	}
	if (action == "" && r.Method != "GET") || (action != "" && r.Method != "POST") {
		allow := "GET"
		if action != "" {
			allow = "POST"
		}
		methodNotAllowed(w, allow)
		return
	}
	if h.stopping.Load() || h.downloadRepo == nil {
		fail(w, 503, "not-ready", "Download tracking is unavailable.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if h.storage.Ready(ctx) != nil {
		downloadFailure(w, downloads.ErrUnavailable)
		return
	}
	completed, err := h.storage.SetupCompleted(ctx)
	if err != nil {
		downloadFailure(w, downloads.ErrUnavailable)
		return
	}
	if !completed {
		fail(w, 409, "setup-required", "Complete setup before tracking downloads.")
		return
	}
	if root {
		q, ok := catalogQuery(w, r, "limit", "offset", "movieId")
		if !ok {
			return
		}
		limit, a := boundedInteger(q, "limit", 25, 1, 50)
		offset, b := boundedInteger(q, "offset", 0, 0, 10000)
		movie, c := boundedInteger(q, "movieId", 0, 1, 9007199254740991)
		if !a || !b || !c {
			downloadFailure(w, downloads.ErrValidation)
			return
		}
		if r.ContentLength != 0 || len(r.TransferEncoding) > 0 {
			fail(w, 400, "invalid-request", "Reads accept no body.")
			return
		}
		value, err := h.downloadRepo.ListDownloads(ctx, limit, offset, int64(movie))
		if err != nil {
			downloadFailure(w, err)
			return
		}
		writeJSON(w, 200, value)
		return
	}
	if _, ok := catalogQuery(w, r); !ok {
		return
	}
	if action == "" {
		if r.ContentLength != 0 || len(r.TransferEncoding) > 0 {
			fail(w, 400, "invalid-request", "Reads accept no body.")
			return
		}
		value, err := h.downloadRepo.GetDownload(ctx, id)
		if err != nil {
			downloadFailure(w, err)
			return
		}
		writeJSON(w, 200, value)
		return
	}
	var input struct {
		Revision int `json:"revision"`
	}
	fields, ok := decodeConfiguration(w, r, &input, "revision")
	if !ok {
		return
	}
	if fields["revision"] == nil || input.Revision < 1 {
		downloadFailure(w, downloads.ErrValidation)
		return
	}
	value, err := h.downloadRepo.QueueDownloadCheck(ctx, id, input.Revision)
	if err != nil {
		downloadFailure(w, err)
		return
	}
	writeJSON(w, 202, value)
}
func downloadFailure(w http.ResponseWriter, err error) {
	var code downloads.Error
	if !errors.As(err, &code) {
		code = downloads.ErrUnavailable
	}
	status, message := 409, "Download state changed. Refresh before continuing."
	switch code {
	case downloads.ErrNotFound:
		status = 404
		message = "Download task does not exist."
	case downloads.ErrUnavailable:
		status = 503
		message = "Local download tracking is unavailable."
	case downloads.ErrValidation:
		status = 422
		message = "Provide valid pagination and revision fields."
	case downloads.ErrCooldown:
		message = "A scheduled check is waiting. Refresh after its due time."
	}
	fail(w, status, string(code), message)
}
