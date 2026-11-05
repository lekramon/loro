package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

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
	data, err := io.ReadAll(r.Body)
	var large *http.MaxBytesError
	if errors.As(err, &large) {
		fail(w, 413, "request-too-large", "Request exceeds 64 KiB.")
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	first, decodeErr := decoder.Token()
	fields := map[string]json.RawMessage{}
	if err != nil || !utf8.Valid(data) || decodeErr != nil || first != json.Delim('{') {
		fail(w, 400, "invalid-request", "Provide a source configuration object.")
		return
	}
	for decoder.More() {
		key, e := decoder.Token()
		if e != nil {
			decodeErr = e
			break
		}
		name, ok := key.(string)
		if !ok || fields[name] != nil || (name != "revision" && name != "name" && name != "url" && name != "apiKey" && name != "clearApiKey") {
			decodeErr = errors.New("invalid field")
			break
		}
		var value json.RawMessage
		if e = decoder.Decode(&value); e != nil {
			decodeErr = e
			break
		}
		fields[name] = value
	}
	if decodeErr == nil {
		_, decodeErr = decoder.Token()
	}
	if decodeErr == nil {
		if _, e := decoder.Token(); e != io.EOF {
			decodeErr = errors.New("trailing input")
		}
	}
	var input sources.Input
	for _, raw := range fields {
		if bytes.Equal(raw, []byte("null")) {
			decodeErr = errors.New("null field")
		}
	}
	if decodeErr != nil || json.Unmarshal(data, &input) != nil {
		fail(w, 400, "invalid-request", "Use unique recognized source fields with the expected types.")
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
		fail(w, 409, "source-test-in-progress", "A source test is already running.")
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
			message = "The source returned invalid capabilities."
		case "source-rate-limited":
			status = 429
			message = "The source is rate limiting requests."
			w.Header().Set("Retry-After", strconv.Itoa(provider.RetryAfter))
		}
		fail(w, status, provider.Code, message)
	}
}
