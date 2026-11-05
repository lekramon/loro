package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/lekramon/piratarr/internal/downloadclient"
)

func (h *Handler) downloadClientAPI(w http.ResponseWriter, r *http.Request) {
	test := r.URL.Path == "/api/v1/download-client/test"
	if test && r.Method != "POST" {
		methodNotAllowed(w, "POST")
		return
	}
	if !test && r.Method != "GET" && r.Method != "PUT" {
		methodNotAllowed(w, "GET, PUT")
		return
	}
	if h.stopping.Load() || h.downloadClient == nil {
		fail(w, 503, "not-ready", "Download client configuration is unavailable.")
		return
	}
	if _, ok := catalogQuery(w, r); !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if h.storage.Ready(ctx) != nil {
		fail(w, 503, "storage-unavailable", "Local download client storage is unavailable.")
		return
	}
	if r.Method == "GET" {
		if r.ContentLength != 0 || len(r.TransferEncoding) > 0 {
			fail(w, 400, "invalid-request", "Configuration reads accept no body.")
			return
		}
		config, err := h.downloadClient.Get(ctx)
		if err != nil {
			downloadClientFailure(w, err)
			return
		}
		writeJSON(w, 200, config)
		return
	}
	var input downloadclient.Input
	if test {
		if _, ok := decodeConfiguration(w, r, &struct{}{}); !ok {
			return
		}
	} else {
		fields, ok := decodeConfiguration(w, r, &input, "revision", "url", "username", "password")
		if !ok {
			return
		}
		if fields["revision"] == nil {
			downloadClientFailure(w, downloadclient.ErrValidation)
			return
		}
	}
	completed, err := h.storage.SetupCompleted(ctx)
	if err != nil {
		downloadClientFailure(w, downloadclient.ErrUnavailable)
		return
	}
	if !completed {
		fail(w, 409, "setup-required", "Complete setup before configuring or testing a download client.")
		return
	}
	if test {
		result, err := h.downloadClient.Test(ctx)
		if err != nil {
			downloadClientFailure(w, err)
			return
		}
		writeJSON(w, 200, result)
		return
	}
	saved, err := h.downloadClient.Save(ctx, input)
	if err != nil {
		downloadClientFailure(w, err)
		return
	}
	writeJSON(w, 200, saved)
}
func downloadClientFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, downloadclient.ErrChanged):
		fail(w, 409, "download-client-changed", "Download client configuration changed. Reload before continuing.")
	case errors.Is(err, downloadclient.ErrValidation):
		fail(w, 422, "download-client-validation-failed", "Check the base URL and username. Initial setup or changes to either require a nonempty password.")
	case errors.Is(err, downloadclient.ErrUnconfigured):
		fail(w, 503, "download-client-unconfigured", "Save a download client before testing it.")
	case errors.Is(err, downloadclient.ErrBusy):
		fail(w, 409, "download-client-test-in-progress", "A download client test is already running.")
	case errors.Is(err, downloadclient.ErrUnavailable):
		fail(w, 503, "storage-unavailable", "Local download client storage is unavailable.")
	default:
		var provider *downloadclient.ProviderError
		if !errors.As(err, &provider) {
			fail(w, 503, "download-client-unavailable", "The download client is unavailable.")
			return
		}
		status, message := 503, "The download client is unavailable."
		switch provider.Code {
		case "download-client-authentication-failed":
			message = "qBittorrent rejected authentication. Check credentials and WebUI access restrictions."
		case "download-client-unsupported-version":
			message = "Use stable qBittorrent 5.x with WebUI API 2.x."
		case "download-client-invalid-response":
			status = 502
			message = "The endpoint returned an invalid qBittorrent response."
		case "download-client-timeout":
			status = 504
			message = "The download client timed out."
		case "download-client-rate-limited":
			status = 429
			message = "The download client is rate limiting requests."
			w.Header().Set("Retry-After", strconv.Itoa(provider.RetryAfter))
		}
		fail(w, status, provider.Code, message)
	}
}
