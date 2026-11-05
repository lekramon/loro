package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/lekramon/piratarr/internal/settings"
)

func decodeSetup(w http.ResponseWriter, r *http.Request) (settings.Input, bool) {
	var input settings.Input
	data, err := io.ReadAll(r.Body)
	var sizeError *http.MaxBytesError
	if errors.As(err, &sizeError) {
		fail(w, 413, "request-too-large", "Request exceeds 64 KiB.")
		return input, false
	}
	data = bytes.TrimSpace(data)
	if err != nil || !utf8.Valid(data) || len(data) == 0 || data[0] != '{' {
		fail(w, 400, "invalid-request", "Provide one valid JSON object.")
		return input, false
	}
	tokens := json.NewDecoder(bytes.NewReader(data))
	if err = uniqueJSON(tokens, "", 0); err == nil {
		_, err = tokens.Token()
		if errors.Is(err, io.EOF) {
			err = nil
		} else {
			err = errors.New("trailing JSON")
		}
	}
	if err != nil {
		fail(w, 400, "invalid-request", "JSON must have unique, recognized fields and no trailing values.")
		return input, false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		fail(w, 400, "invalid-request", "JSON fields must use the expected types.")
		return input, false
	}
	return input, true
}

// Token-level validation catches duplicate keys before encoding/json can
// silently discard earlier values, and enforces case-sensitive contract keys.
func uniqueJSON(decoder *json.Decoder, location string, depth int) error {
	if depth > 100 {
		return errors.New("JSON nesting limit exceeded")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		allowed := map[string]map[string]bool{
			"":            {"directories": true, "profile": true},
			"directories": {"downloadDirectory": true, "movieDirectory": true},
			"profile":     {"acceptedResolutions": true, "preferredResolution": true, "audioPreferences": true},
		}[location]
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok || seen[key] || !allowed[key] {
				return errors.New("duplicate or unknown JSON field")
			}
			seen[key] = true
			child := key
			if location != "" {
				child = location + "." + key
			}
			if err := uniqueJSON(decoder, child, depth+1); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("unterminated object")
		}
	case '[':
		for decoder.More() {
			if err := uniqueJSON(decoder, location+"[]", depth+1); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("unterminated array")
		}
	default:
		return errors.New("unexpected delimiter")
	}
	return nil
}

func settingsFailure(w http.ResponseWriter, err error) {
	var validation *settings.ValidationError
	switch {
	case errors.As(err, &validation):
		writeJSON(w, 422, apiError{Code: "validation-failed", Message: "Correct the highlighted settings.", FieldErrors: validation.Fields})
	case errors.Is(err, settings.ErrAlreadyCompleted):
		fail(w, 409, "setup-already-completed", "Setup has already been completed. Reload to view the saved settings.")
	case errors.Is(err, settings.ErrSetupRequired):
		fail(w, 409, "setup-required", "Complete first-run setup before reading settings.")
	case errors.Is(err, settings.ErrUnavailable):
		fail(w, 503, "storage-unavailable", "Local settings storage is unavailable. Try again when storage is accessible.")
	default:
		fail(w, 500, "internal-error", "Settings could not be saved or loaded.")
	}
}
