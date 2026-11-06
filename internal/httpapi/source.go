package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/lekramon/piratarr/internal/sources"
)

func (h *Handler) sourceAPI(w http.ResponseWriter, r *http.Request) {
	test := r.URL.Path == "/api/v1/source/test"
	if (test && r.Method != "GET") || (!test && r.Method != "GET" && r.Method != "PUT") {
		allow := "GET, PUT"
		if test {
			allow = "GET"
		}
		methodNotAllowed(w, allow)
		return
	}
	if h.stopping.Load() || h.source == nil {
		fail(w, 503, "not-ready", "Source configuration is unavailable.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if h.storage.Ready(ctx) != nil {
		fail(w, 503, "storage-unavailable", "Local source storage is unavailable.")
		return
	}
	if _, ok := catalogQuery(w, r); !ok {
		return
	}
	if test {
		if r.ContentLength != 0 || len(r.TransferEncoding) > 0 {
			fail(w, 400, "invalid-request", "Source tests do not accept a body.")
			return
		}
		result, err := h.source.Test(ctx)
		if err != nil {
			sourceFailure(w, err)
			return
		}
		writeJSON(w, 200, result)
		return
	}
	if r.Method == "GET" {
		config, err := h.source.Get(ctx)
		if err != nil {
			sourceFailure(w, err)
			return
		}
		writeJSON(w, 200, config)
		return
	}
	var input sources.Input
	fields, ok := decodeConfiguration(w, r, &input, "revision", "name", "url", "apiKey", "clearApiKey")
	if !ok {
		return
	}
	if fields["revision"] == nil {
		fail(w, 422, "source-validation-failed", "Provide the current source revision.")
		return
	}
	completed, err := h.storage.SetupCompleted(ctx)
	if err != nil {
		sourceFailure(w, sources.ErrUnavailable)
		return
	}
	if !completed {
		fail(w, 409, "setup-required", "Complete setup before configuring a source.")
		return
	}
	saved, err := h.source.Save(ctx, input)
	if err != nil {
		sourceFailure(w, err)
		return
	}
	writeJSON(w, 200, saved)
}
func sourceFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sources.ErrChanged):
		fail(w, 409, "source-changed", "Source configuration changed. Reload it before trying again.")
	case errors.Is(err, sources.ErrBusy):
		fail(w, 409, "source-test-in-progress", "A source operation is already running.")
	case errors.Is(err, sources.ErrValidation):
		fail(w, 422, "source-validation-failed", "Check source name, endpoint and credential fields. Endpoint changes require a new key or explicit clearing.")
	case errors.Is(err, sources.ErrUnconfigured):
		fail(w, 503, "source-unconfigured", "Configure a source before testing it.")
	case errors.Is(err, sources.ErrUnavailable):
		fail(w, 503, "storage-unavailable", "Local source storage is unavailable.")
	default:
		var provider *sources.ProviderError
		if !errors.As(err, &provider) {
			fail(w, 503, "source-unavailable", "The source is unavailable.")
			return
		}
		status := 503
		message := "The source is unavailable."
		switch provider.Code {
		case "source-authentication-failed":
			message = "The source rejected its credentials."
		case "source-timeout":
			status = 504
			message = "The source timed out."
		case "source-invalid-response":
			status = 502
			message = "The source returned an invalid response."
		case "source-challenge-required":
			status = 502
			message = "The source requires browser verification."
		case "source-rate-limited":
			status = 429
			message = "The source is rate limiting requests."
			w.Header().Set("Retry-After", strconv.Itoa(provider.RetryAfter))
		}
		fail(w, status, provider.Code, message)
	}
}
