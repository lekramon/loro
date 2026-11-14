package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/importlists"
)

func (h *Handler) importListAPI(w http.ResponseWriter, r *http.Request) {
	if h.stopping.Load() || h.importLists == nil {
		fail(w, 503, "import-list-unavailable", "Import list service is unavailable.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	completed, err := h.storage.SetupCompleted(ctx)
	if err != nil {
		fail(w, 503, "storage-unavailable", "Local storage is unavailable.")
		return
	}
	if !completed {
		fail(w, 409, "setup-required", "Complete first-run setup before importing lists.")
		return
	}
	if r.URL.Path == "/api/v1/import-lists" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, "GET")
			return
		}
		items, listErr := h.importLists.Repo.ListImportLists(ctx)
		if listErr != nil {
			importListFailure(w, listErr)
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
		return
	}
	if r.URL.Path == "/api/v1/import-lists/preview" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, "POST")
			return
		}
		var input importlists.PreviewInput
		if _, ok := decodeConfiguration(w, r, &input, "source", "listKey", "name", "csv", "language"); !ok {
			return
		}
		preview, previewErr := h.importLists.Preview(ctx, input)
		if previewErr != nil {
			importListFailure(w, previewErr)
			return
		}
		writeJSON(w, 200, preview)
		return
	}
	const prefix = "/api/v1/import-lists/previews/"
	if !strings.HasPrefix(r.URL.Path, prefix) || !strings.HasSuffix(r.URL.Path, "/commit") {
		fail(w, 404, "not-found", "Import list resource does not exist.")
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), "/commit")
	if strings.Contains(id, "/") {
		fail(w, 404, "not-found", "Import list preview does not exist.")
		return
	}
	var input importlists.CommitInput
	if _, ok := decodeConfiguration(w, r, &input, "revision"); !ok {
		return
	}
	result, commitErr := h.importLists.Commit(ctx, id, input.Revision)
	if commitErr != nil {
		importListFailure(w, commitErr)
		return
	}
	writeJSON(w, 200, result)
}

func importListFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, importlists.ErrInvalid):
		fail(w, 422, "validation-failed", "Provide a supported bounded CSV export and valid list identity.")
	case errors.Is(err, importlists.ErrNotFound):
		fail(w, 404, "import-list-not-found", "Import list preview does not exist.")
	case errors.Is(err, importlists.ErrChanged):
		fail(w, 409, "import-list-changed", "Import list preview changed. Refresh before continuing.")
	case errors.Is(err, importlists.ErrUnavailable):
		fail(w, 503, "import-list-unavailable", "Import list storage is unavailable.")
	case errors.Is(err, catalog.ErrNotFound):
		fail(w, 404, "metadata-not-found", "Matched metadata does not exist.")
	default:
		catalogFailure(w, err)
	}
}
