package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/lekramon/piratarr/internal/sources"
)

func (h *Handler) releaseSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		methodNotAllowed(w, "GET")
		return
	}
	if h.stopping.Load() || h.source == nil || h.catalog == nil || h.acquisitions == nil {
		fail(w, 503, "not-ready", "Release search is unavailable.")
		return
	}
	parameters, ok := catalogQuery(w, r, "movieId", "limit", "offset", "category")
	if !ok {
		return
	}
	if r.ContentLength != 0 || len(r.TransferEncoding) > 0 {
		fail(w, 400, "invalid-request", "Release search accepts no body.")
		return
	}
	id, err := strconv.ParseInt(parameters.Get("movieId"), 10, 64)
	limit, vl := boundedInteger(parameters, "limit", 25, 1, 50)
	offset, vo := boundedInteger(parameters, "offset", 0, 0, 10000)
	category, vc := boundedInteger(parameters, "category", 0, 1, 1000000)
	if err != nil || id < 1 || !vl || !vo || !vc {
		fail(w, 422, "validation-failed", "Provide a catalog movie ID, limit 1–50, offset 0–10000 and a discovered category.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if h.storage.Ready(ctx) != nil {
		fail(w, 503, "storage-unavailable", "Local storage is unavailable.")
		return
	}
	completed, err := h.storage.SetupCompleted(ctx)
	if err != nil {
		sourceFailure(w, sources.ErrUnavailable)
		return
	}
	if !completed {
		fail(w, 409, "setup-required", "Complete setup before searching releases.")
		return
	}
	profile, err := h.storage.GetSettings(ctx)
	if err != nil {
		sourceFailure(w, sources.ErrUnavailable)
		return
	}
	movie, err := h.catalog.Repo.MovieByID(ctx, id)
	if err != nil {
		catalogFailure(w, err)
		return
	}
	page, err := h.source.Search(ctx, movie, profile, limit, offset, category)
	if err != nil {
		switch {
		case errors.Is(err, sources.ErrCapabilities):
			fail(w, 409, "source-capabilities-required", "Test the saved source before searching.")
		case errors.Is(err, sources.ErrUnsupported):
			fail(w, 409, "source-search-unsupported", "The source does not declare supported movie or text search.")
		case errors.Is(err, sources.ErrValidation):
			fail(w, 422, "validation-failed", "Choose a discovered category and valid pagination.")
		default:
			sourceFailure(w, err)
		}
		return
	}
	current, err := h.storage.GetSettings(ctx)
	if err != nil {
		sourceFailure(w, sources.ErrUnavailable)
		return
	}
	if current.Revision != profile.Revision {
		fail(w, 409, "preferences-changed", "Preferences changed during search. Search again.")
		return
	}
	page, err = h.acquisitions.Remember(ctx, page, movie, profile)
	if err != nil {
		acquisitionFailure(w, err)
		return
	}
	writeJSON(w, 200, page)
}
