package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/lekramon/piratarr/internal/series"
	"github.com/lekramon/piratarr/internal/sources"
)

func (h *Handler) episodeSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return
	}
	if h.stopping.Load() || h.source == nil || h.seriesRepo == nil || h.acquisitions == nil {
		fail(w, 503, "not-ready", "Episode search is unavailable.")
		return
	}
	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(75 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		fail(w, 503, "source-unavailable", "Unable to start episode search.")
		return
	}
	var input sources.EpisodeSearch
	if _, ok := decodeConfiguration(w, r, &input, "episodeId", "query", "seasonNumber", "episodeNumber", "absoluteNumber"); !ok {
		return
	}
	input.Query = strings.TrimSpace(input.Query)
	ctx, cancel := context.WithTimeout(r.Context(), sources.SearchTimeout)
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
		fail(w, 409, "setup-required", "Complete setup before searching episodes.")
		return
	}
	episode, err := h.seriesRepo.GetEpisode(ctx, input.EpisodeID)
	if err != nil {
		if errors.Is(err, series.ErrNotFound) {
			fail(w, 404, "episode-not-found", "Episode was not found.")
		} else {
			fail(w, 503, "storage-unavailable", "Episode catalog storage is unavailable.")
		}
		return
	}
	profile, err := h.storage.GetSettings(ctx)
	if err != nil {
		sourceFailure(w, sources.ErrUnavailable)
		return
	}
	result, err := h.source.SearchEpisode(ctx, input, profile)
	if err != nil {
		switch {
		case errors.Is(err, sources.ErrCapabilities):
			fail(w, 409, "source-capabilities-required", "Test the saved source before searching.")
		case errors.Is(err, sources.ErrUnsupported):
			fail(w, 409, "source-search-unsupported", "The source does not declare text search.")
		case errors.Is(err, sources.ErrValidation):
			fail(w, 422, "validation-failed", "Provide a valid episode search.")
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
	result, err = h.acquisitions.RememberEpisode(ctx, result, episode, profile)
	if err != nil {
		acquisitionFailure(w, err)
		return
	}
	writeJSON(w, 200, result)
}
