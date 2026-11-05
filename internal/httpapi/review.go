package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/lekramon/piratarr/internal/library"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (h *Handler) reviewAPI(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/library/files/"), "/")
	if len(parts) != 2 || parts[1] != "review" {
		fail(w, 404, "not-found", "Library resource was not found.")
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id < 1 {
		fail(w, 404, "not-found", "Library resource was not found.")
		return
	}
	if r.Method != "POST" {
		methodNotAllowed(w, "POST")
		return
	}
	if h.stopping.Load() || h.library == nil {
		fail(w, 503, "not-ready", "Library review is unavailable.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if h.storage.Ready(ctx) != nil {
		fail(w, 503, "storage-unavailable", "Local storage is unavailable.")
		return
	}
	if _, ok := catalogQuery(w, r); !ok {
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
	if err != nil || decodeErr != nil || first != json.Delim('{') {
		fail(w, 400, "invalid-request", "Provide a review JSON object.")
		return
	}
	for decoder.More() {
		key, e := decoder.Token()
		if e != nil {
			decodeErr = e
			break
		}
		name, ok := key.(string)
		if !ok || fields[name] != nil || (name != "revision" && name != "action" && name != "movieId" && name != "confirmIdentity") {
			decodeErr = errors.New("invalid field")
			break
		}
		var raw json.RawMessage
		if e = decoder.Decode(&raw); e != nil {
			decodeErr = e
			break
		}
		fields[name] = raw
	}
	if decodeErr == nil {
		_, decodeErr = decoder.Token()
	}
	if decodeErr == nil {
		if _, e := decoder.Token(); e != io.EOF {
			decodeErr = errors.New("trailing value")
		}
	}
	var input library.Review
	if decodeErr != nil || json.Unmarshal(data, &input) != nil || bytes.Equal(fields["revision"], []byte("null")) || bytes.Equal(fields["action"], []byte("null")) {
		fail(w, 400, "invalid-request", "Use unique recognized review fields with the expected types.")
		return
	}
	valid := input.Revision > 0 && (input.Action == "associate" || input.Action == "ignore" || input.Action == "reopen")
	if input.Action == "associate" {
		valid = valid && input.MovieID != nil && *input.MovieID > 0 && input.ConfirmIdentity
	} else {
		valid = valid && fields["movieId"] == nil && fields["confirmIdentity"] == nil
	}
	if !valid {
		fail(w, 422, "validation-failed", "Choose a valid action and revision; association requires a selected movie and explicit identity confirmation.")
		return
	}
	entry, err := h.library.Review(ctx, id, input)
	if err != nil {
		libraryFailure(w, err)
		return
	}
	writeJSON(w, 200, entry)
}
