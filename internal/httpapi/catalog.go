package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lekramon/piratarr/internal/catalog"
)

func (h *Handler) catalogAPI(w http.ResponseWriter, r *http.Request) bool {
	route := r.URL.Path
	imageRoute := strings.HasPrefix(route, catalog.ImagePrefix)
	detailRoute := strings.HasPrefix(route, "/api/v1/catalog/movies/")
	if route != "/api/v1/metadata/test" && route != "/api/v1/about" && route != "/api/v1/metadata/status" && route != "/api/v1/metadata/search" && route != "/api/v1/catalog/movies" && !imageRoute && !detailRoute {
		return false
	}
	allow := "GET"
	if route == "/api/v1/catalog/movies" {
		allow = "GET, POST"
	}
	if r.Method != "GET" && (route != "/api/v1/catalog/movies" || r.Method != "POST") {
		methodNotAllowed(w, allow)
		return true
	}
	w.Header().Set("Link", `</api/v1/about>; rel="describedby"`)
	if route == "/api/v1/about" {
		writeJSON(w, 200, map[string]string{"name": "Lôro", "attribution": catalog.Attribution, "providerUrl": "https://www.themoviedb.org", "logoUrl": "/tmdb.svg"})
		return true
	}
	if h.stopping.Load() {
		fail(w, 503, "not-ready", "The service is stopping.")
		return true
	}
	if route == "/api/v1/metadata/status" {
		configured := h.catalog != nil && h.catalog.Provider.Configured()
		writeJSON(w, 200, map[string]any{"provider": "tmdb", "configured": configured})
		return true
	}
	if h.catalog == nil {
		fail(w, 503, "catalog-unavailable", "The catalog service is unavailable.")
		return true
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if h.storage.Ready(ctx) != nil {
		fail(w, 503, "storage-unavailable", "Local catalog storage is unavailable.")
		return true
	}
	switch {
	case route == "/api/v1/metadata/test":
		// Bypass local search caches so success proves an authenticated request now.
		_, err := h.catalog.Provider.Search(ctx, "Inception", 0, 1, "en-US")
		if err != nil {
			catalogFailure(w, err)
			return true
		}
		writeJSON(w, 200, map[string]any{"provider": "tmdb", "connected": true})
	case route == "/api/v1/metadata/search":
		parameters, ok := catalogQuery(w, r, "query", "year", "page", "language")
		if !ok {
			return true
		}
		query := strings.TrimSpace(parameters.Get("query"))
		language, validLanguage := catalog.Language(parameters.Get("language"))
		page, validPage := boundedInteger(parameters, "page", 1, 1, 500)
		year, validYear := boundedInteger(parameters, "year", 0, 1870, 2100)
		if !utf8.ValidString(query) || utf8.RuneCountInString(query) < 1 || utf8.RuneCountInString(query) > 200 || !validLanguage || !validPage || !validYear {
			fail(w, 422, "validation-failed", "Provide a query of 1–200 characters, a supported language, a valid year and page 1–500.")
			return true
		}
		result, err := h.catalog.Search(ctx, query, year, page, language)
		if err != nil {
			catalogFailure(w, err)
			return true
		}
		writeJSON(w, 200, result)
	case route == "/api/v1/catalog/movies" && r.Method == "POST":
		id, language, ok := decodeRegistration(w, r)
		if !ok {
			return true
		}
		completed, err := h.storage.SetupCompleted(ctx)
		if err != nil {
			fail(w, 503, "storage-unavailable", "Local catalog storage is unavailable.")
			return true
		}
		if !completed {
			fail(w, 409, "setup-required", "Complete first-run setup before registering movies.")
			return true
		}
		movie, created, err := h.catalog.Register(ctx, id, language)
		if err != nil {
			catalogFailure(w, err)
			return true
		}
		status := 200
		if created {
			status = 201
			w.Header().Set("Location", "/api/v1/catalog/movies/"+strconv.FormatInt(movie.ID, 10))
		}
		writeJSON(w, status, struct {
			Movie   catalog.Movie `json:"movie"`
			Created bool          `json:"created"`
		}{movie, created})
	case route == "/api/v1/catalog/movies":
		parameters, ok := catalogQuery(w, r, "limit", "offset", "q")
		if !ok {
			return true
		}
		limit, validLimit := boundedInteger(parameters, "limit", 50, 1, 100)
		offset, validOffset := boundedInteger(parameters, "offset", 0, 0, 1000000)
		query := strings.TrimSpace(parameters.Get("q"))
		if !validLimit || !validOffset || !utf8.ValidString(query) || utf8.RuneCountInString(query) > 200 {
			fail(w, 422, "validation-failed", "Provide a limit of 1–100, a valid offset and a title filter up to 200 characters.")
			return true
		}
		result, err := h.catalog.Repo.ListMovies(ctx, limit, offset, query)
		if err != nil {
			catalogFailure(w, err)
			return true
		}
		writeJSON(w, 200, result)
	case detailRoute:
		id, err := strconv.ParseInt(strings.TrimPrefix(route, "/api/v1/catalog/movies/"), 10, 64)
		if err != nil || id < 1 {
			fail(w, 404, "not-found", "Catalog movie does not exist.")
			return true
		}
		movie, err := h.catalog.Repo.MovieByID(ctx, id)
		if err != nil {
			catalogFailure(w, err)
			return true
		}
		writeJSON(w, 200, movie)
	case imageRoute:
		parts := strings.Split(strings.TrimPrefix(route, catalog.ImagePrefix), "/")
		if len(parts) != 2 {
			fail(w, 404, "not-found", "Artwork does not exist.")
			return true
		}
		data, mime, err := h.catalog.Image(ctx, parts[0], parts[1])
		if err != nil {
			catalogFailure(w, err)
			return true
		}
		w.Header().Set("Content-Type", mime)
		w.Header().Set("Cache-Control", "private, max-age=86400")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}
	return true
}
func catalogQuery(w http.ResponseWriter, r *http.Request, allowed ...string) (url.Values, bool) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		fail(w, 400, "invalid-request", "Provide valid query parameters.")
		return nil, false
	}
	for key, items := range values {
		found := false
		for _, name := range allowed {
			if key == name {
				found = true
			}
		}
		if !found || len(items) != 1 {
			fail(w, 422, "validation-failed", "Use recognized query parameters once each.")
			return nil, false
		}
	}
	return values, true
}
func boundedInteger(values url.Values, key string, fallback, min, max int) (int, bool) {
	items, present := values[key]
	if !present {
		return fallback, true
	}
	if len(items) != 1 {
		return 0, false
	}
	number, err := strconv.Atoi(items[0])
	return number, err == nil && number >= min && number <= max
}
func decodeRegistration(w http.ResponseWriter, r *http.Request) (int64, string, bool) {
	data, err := io.ReadAll(r.Body)
	var large *http.MaxBytesError
	if errors.As(err, &large) {
		fail(w, 413, "request-too-large", "Request exceeds 64 KiB.")
		return 0, "", false
	}
	data = bytes.TrimSpace(data)
	if err != nil || !utf8.Valid(data) || len(data) == 0 || data[0] != '{' {
		fail(w, 400, "invalid-request", "Provide one valid JSON object.")
		return 0, "", false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	_, err = decoder.Token()
	seen := map[string]bool{}
	fields := map[string]json.RawMessage{}
	if err == nil {
		for decoder.More() {
			var token any
			token, err = decoder.Token()
			if err != nil {
				break
			}
			key, ok := token.(string)
			if !ok || seen[key] || (key != "tmdbId" && key != "language") {
				err = errors.New("unknown or duplicate field")
				break
			}
			seen[key] = true
			var value json.RawMessage
			err = decoder.Decode(&value)
			if err != nil {
				break
			}
			fields[key] = value
		}
	}
	if err == nil {
		_, err = decoder.Token()
	}
	if err == nil {
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			err = errors.New("trailing JSON")
		}
	}
	var input struct {
		TMDBID   int64  `json:"tmdbId"`
		Language string `json:"language"`
	}
	if err != nil || json.Unmarshal(data, &input) != nil || bytes.Equal(fields["tmdbId"], []byte("null")) || bytes.Equal(fields["language"], []byte("null")) {
		fail(w, 400, "invalid-request", "JSON must contain unique recognized fields with the expected types and no trailing values.")
		return 0, "", false
	}
	language, valid := catalog.Language(input.Language)
	if input.TMDBID < 1 || input.TMDBID > 2147483647 || !valid {
		fail(w, 422, "validation-failed", "Provide a positive TMDB movie ID and a supported metadata language.")
		return 0, "", false
	}
	return input.TMDBID, language, true
}
func catalogFailure(w http.ResponseWriter, err error) {
	var provider *catalog.ProviderError
	switch {
	case errors.Is(err, catalog.ErrNotFound):
		fail(w, 404, "not-found", "Catalog movie or artwork does not exist.")
	case errors.Is(err, catalog.ErrUnavailable):
		fail(w, 503, "storage-unavailable", "Local catalog storage is unavailable.")
	case errors.As(err, &provider):
		status := 503
		message := "Movie metadata is temporarily unavailable."
		switch provider.Code {
		case "metadata-unconfigured":
			message = "Application metadata credentials are unavailable."
		case "metadata-authentication-failed":
			message = "The metadata provider rejected the application credential."
		case "metadata-rate-limited":
			status = 429
			message = "The metadata provider is rate limiting requests."
			w.Header().Set("Retry-After", strconv.Itoa(provider.RetryAfter))
		case "metadata-timeout":
			status = 504
			message = "The metadata provider timed out."
		case "metadata-not-found":
			status = 404
			message = "The metadata provider could not find this movie or artwork."
		case "metadata-invalid-response":
			status = 502
			message = "The metadata provider returned an invalid response."
		}
		fail(w, status, provider.Code, message)
	default:
		fail(w, 500, "internal-error", "Catalog data could not be saved or loaded.")
	}
}
