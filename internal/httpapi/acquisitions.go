package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/lekramon/piratarr/internal/acquisitions"
)

func (h *Handler) acquisitionAPI(w http.ResponseWriter, r *http.Request) {
	root := r.URL.Path == "/api/v1/acquisitions"
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/acquisitions/"), "/")
	id, action := "", ""
	if !root {
		if len(parts) < 1 || len(parts) > 2 || !acquisitions.ValidID(parts[0]) {
			fail(w, 404, "acquisition-not-found", "Acquisition does not exist.")
			return
		}
		id = parts[0]
		if len(parts) == 2 {
			action = parts[1]
			if action != "prepare" && action != "cancel" {
				fail(w, 404, "not-found", "Resource does not exist.")
				return
			}
		}
	}
	allow := "GET"
	if root {
		allow = "GET, POST"
	} else if action != "" {
		allow = "POST"
	}
	if r.Method != "GET" && r.Method != "POST" || r.Method == "POST" && !root && action == "" || r.Method == "GET" && action != "" {
		methodNotAllowed(w, allow)
		return
	}
	if h.stopping.Load() || h.acquisitions == nil {
		fail(w, 503, "not-ready", "Acquisitions are unavailable.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if h.storage.Ready(ctx) != nil {
		acquisitionFailure(w, acquisitions.ErrUnavailable)
		return
	}
	completed, err := h.storage.SetupCompleted(ctx)
	if err != nil {
		acquisitionFailure(w, acquisitions.ErrUnavailable)
		return
	}
	if !completed {
		fail(w, 409, "setup-required", "Complete setup before selecting releases.")
		return
	}
	if r.Method == "GET" && root {
		query, ok := catalogQuery(w, r, "limit", "offset", "movieId")
		if !ok {
			return
		}
		limit, a := boundedInteger(query, "limit", 25, 1, 50)
		offset, b := boundedInteger(query, "offset", 0, 0, 10000)
		movie, c := boundedInteger(query, "movieId", 0, 1, 9007199254740991)
		if !a || !b || !c {
			acquisitionFailure(w, acquisitions.ErrValidation)
			return
		}
		if r.ContentLength != 0 || len(r.TransferEncoding) > 0 {
			fail(w, 400, "invalid-request", "Acquisition reads accept no body.")
			return
		}
		page, err := h.acquisitions.Repo.ListAcquisitions(ctx, limit, offset, int64(movie))
		if err != nil {
			acquisitionFailure(w, err)
			return
		}
		writeJSON(w, 200, page)
		return
	}
	if _, ok := catalogQuery(w, r); !ok {
		return
	}
	if r.Method == "GET" {
		if r.ContentLength != 0 || len(r.TransferEncoding) > 0 {
			fail(w, 400, "invalid-request", "Acquisition reads accept no body.")
			return
		}
		record, err := h.acquisitions.Repo.GetAcquisition(ctx, id)
		if err != nil {
			acquisitionFailure(w, err)
			return
		}
		writeJSON(w, 200, record.Acquisition)
		return
	}
	if root {
		var input acquisitions.Selection
		fields, ok := decodeConfiguration(w, r, &input, "observationId", "idempotencyKey", "confirmOverride")
		if !ok {
			return
		}
		if fields["observationId"] == nil || fields["idempotencyKey"] == nil || fields["confirmOverride"] == nil {
			acquisitionFailure(w, acquisitions.ErrValidation)
			return
		}
		value, created, err := h.acquisitions.Select(ctx, input)
		if err != nil {
			acquisitionFailure(w, err)
			return
		}
		status := 200
		if created {
			status = 201
			w.Header().Set("Location", "/api/v1/acquisitions/"+value.ID)
		}
		writeJSON(w, status, struct {
			Acquisition acquisitions.Acquisition `json:"acquisition"`
			Created     bool                     `json:"created"`
		}{value, created})
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
		acquisitionFailure(w, acquisitions.ErrValidation)
		return
	}
	var value acquisitions.Acquisition
	if action == "prepare" {
		value, err = h.acquisitions.Prepare(ctx, id, input.Revision)
	} else {
		value, err = h.acquisitions.Repo.CancelAcquisition(ctx, id, input.Revision, time.Now().UTC().Format(time.RFC3339Nano))
	}
	if err != nil {
		acquisitionFailure(w, err)
		return
	}
	writeJSON(w, 200, value)
}
func acquisitionFailure(w http.ResponseWriter, err error) {
	var domain acquisitions.Error
	if !errors.As(err, &domain) {
		domain = acquisitions.ErrUnavailable
	}
	status, message := 409, "Acquisition state changed. Refresh before continuing."
	switch domain {
	case acquisitions.ErrUnavailable:
		status = 503
		message = "Local acquisition storage is unavailable."
	case acquisitions.ErrNotFound, acquisitions.ErrObservationNotFound:
		status = 404
		message = "Acquisition or observation does not exist."
	case acquisitions.ErrValidation:
		status = 422
		message = "Provide valid identifiers, pagination and revision fields."
	case acquisitions.ErrExpired:
		message = "This observation expired. Search releases again."
	case acquisitions.ErrObservationChanged:
		message = "Source or preferences changed. Search releases again."
	case acquisitions.ErrConfirmation:
		message = "Explicitly confirm the profile differences and unknown attributes for this acquisition."
	case acquisitions.ErrIdempotency:
		message = "That request key was already used for different selection input."
	case acquisitions.ErrBusy:
		message = "Identity preparation is already running. Refresh and retry shortly."
	}
	fail(w, status, string(domain), message)
}
