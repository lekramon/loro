package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/lekramon/piratarr/internal/operations"
)

func (h *Handler) operationsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { methodNotAllowed(w, "GET"); return }
	if h.stopping.Load() { fail(w, 503, "not-ready", "Operations status is unavailable."); return }
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.storage.Ready(ctx); err != nil { fail(w, 503, "not-ready", "Operations status is unavailable."); return }
	writeJSON(w, 200, operations.Status{State: "ready", Maintenance: "idle"})
}
