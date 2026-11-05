package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/lekramon/piratarr/internal/tmdb"
)

func (h *Handler) metadataCredential(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		methodNotAllowed(w, "PUT")
		return
	}
	if h.stopping.Load() || h.catalog == nil {
		fail(w, 503, "metadata-unavailable", "Metadata configuration is unavailable.")
		return
	}
	// Token-level decoding rejects duplicate keys without ever reporting submitted values.
	data, readErr := io.ReadAll(r.Body)
	var sizeErr *http.MaxBytesError
	if errors.As(readErr, &sizeErr) {
		fail(w, 413, "request-too-large", "Request exceeds 64 KiB.")
		return
	}
	if readErr != nil || !utf8.Valid(data) {
		fail(w, 400, "invalid-request", "Provide valid JSON.")
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	first, err := decoder.Token()
	var value string
	if err != nil || first != json.Delim('{') {
		fail(w, 400, "invalid-request", "Provide a JSON object containing only token.")
		return
	}
	key, err := decoder.Token()
	if err != nil || key != "token" {
		fail(w, 400, "invalid-request", "Provide a JSON object containing only token.")
		return
	}
	if err := decoder.Decode(&value); err != nil {
		fail(w, 400, "invalid-request", "Token must be a string.")
		return
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		fail(w, 400, "invalid-request", "Provide a JSON object containing only token.")
		return
	}
	if _, err := decoder.Token(); err != io.EOF {
		fail(w, 400, "invalid-request", "Provide one JSON object.")
		return
	}
	saver, ok := h.catalog.Provider.(interface{ SaveToken(string) error })
	if !ok {
		fail(w, 503, "metadata-unavailable", "Metadata configuration is unavailable.")
		return
	}
	if err := saver.SaveToken(value); err != nil {
		if errors.Is(err, tmdb.ErrInvalidToken) {
			fail(w, 422, "validation-failed", "Provide a valid TMDB API Read Access Token.")
		} else {
			fail(w, 503, "metadata-unavailable", "Unable to save metadata credentials.")
		}
		return
	}
	writeJSON(w, 200, map[string]any{"provider": "tmdb", "configured": true})
}
