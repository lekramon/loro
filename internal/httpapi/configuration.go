package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"unicode/utf8"
)

// decodeConfiguration preserves duplicate/unknown/null rejection for flat configuration APIs.
func decodeConfiguration(w http.ResponseWriter, r *http.Request, destination any, names ...string) (map[string]json.RawMessage, bool) {
	data, err := io.ReadAll(r.Body)
	var large *http.MaxBytesError
	if errors.As(err, &large) {
		fail(w, 413, "request-too-large", "Request exceeds 64 KiB.")
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	first, decodeErr := decoder.Token()
	fields := map[string]json.RawMessage{}
	allowed := map[string]bool{}
	for _, name := range names {
		allowed[name] = true
	}
	if err != nil || !utf8.Valid(data) || decodeErr != nil || first != json.Delim('{') {
		fail(w, 400, "invalid-request", "Provide a configuration object.")
		return nil, false
	}
	for decoder.More() {
		key, e := decoder.Token()
		if e != nil {
			decodeErr = e
			break
		}
		name, ok := key.(string)
		if !ok || fields[name] != nil || !allowed[name] {
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
	for _, raw := range fields {
		if bytes.Equal(raw, []byte("null")) {
			decodeErr = errors.New("null field")
		}
	}
	if decodeErr != nil || json.Unmarshal(data, destination) != nil {
		fail(w, 400, "invalid-request", "Use unique recognized fields with non-null values and the expected types.")
		return nil, false
	}
	return fields, true
}
