package sources

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/settings"
)

var ErrCapabilities = errors.New("source capabilities required")
var ErrUnsupported = errors.New("source search unsupported")

type ProfileCheck struct {
	Dimension string   `json:"dimension"`
	Status    string   `json:"status"`
	Accepted  []string `json:"accepted"`
}
type Candidate struct {
	ID                  string         `json:"id"`
	Title               string         `json:"title"`
	SizeBytes           *int64         `json:"sizeBytes"`
	Seeders             *int64         `json:"seeders"`
	PublishedAt         *string        `json:"publishedAt"`
	Resolution          *string        `json:"resolution"`
	Format              *string        `json:"format"`
	Container           *string        `json:"container"`
	AudioLanguages      []string       `json:"audioLanguages"`
	PreferredResolution bool           `json:"preferredResolution"`
	AudioPreferenceRank *int           `json:"audioPreferenceRank"`
	Status              string         `json:"status"`
	Checks              []ProfileCheck `json:"checks"`
}
type ReleasePage struct {
	MovieID         int64       `json:"movieId"`
	SourceName      string      `json:"sourceName"`
	SourceRevision  int         `json:"sourceRevision"`
	ProfileRevision int         `json:"profileRevision"`
	Mode            string      `json:"mode"`
	Limit           int         `json:"limit"`
	Offset          int         `json:"offset"`
	Total           *int64      `json:"total"`
	HasMore         bool        `json:"hasMore"`
	NextOffset      *int        `json:"nextOffset"`
	Items           []Candidate `json:"items"`
}

func (s *Service) Search(ctx context.Context, movie catalog.Movie, profile settings.Settings, limit, offset, category int) (ReleasePage, error) {
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	default:
		return ReleasePage{}, ErrBusy
	}
	config, err := s.repo.GetSource(ctx)
	if err != nil {
		return ReleasePage{}, err
	}
	if !config.Configured {
		return ReleasePage{}, ErrUnconfigured
	}
	if config.Capabilities == nil {
		return ReleasePage{}, ErrCapabilities
	}
	caps := config.Capabilities
	if limit < 1 || limit > 50 || offset < 0 || offset > 10000 {
		return ReleasePage{}, ErrValidation
	}
	if category != 0 && !slices.ContainsFunc(caps.Categories, func(c Category) bool { return c.ID == category }) {
		return ReleasePage{}, ErrValidation
	}
	if caps.MaxLimit != nil && limit > *caps.MaxLimit {
		limit = *caps.MaxLimit
	}
	title := strings.TrimSpace(movie.OriginalTitle)
	if title == "" {
		title = movie.Title
	}
	query := url.Values{"extended": {"1"}, "limit": {strconv.Itoa(limit)}, "offset": {strconv.Itoa(offset)}}
	mode := ""
	switch {
	case caps.MovieSearch.Available && slices.Contains(caps.MovieSearch.SupportedParams, "tmdbid"):
		query.Set("t", "movie")
		query.Set("tmdbid", strconv.FormatInt(movie.TMDBID, 10))
		mode = "movie-tmdb"
	case caps.MovieSearch.Available && slices.Contains(caps.MovieSearch.SupportedParams, "q"):
		query.Set("t", "movie")
		query.Set("q", title)
		mode = "movie-query"
		if movie.Year != nil && slices.Contains(caps.MovieSearch.SupportedParams, "year") {
			query.Set("year", strconv.Itoa(*movie.Year))
		}
	case caps.Search.Available && slices.Contains(caps.Search.SupportedParams, "q"):
		if movie.Year != nil {
			title += " " + strconv.Itoa(*movie.Year)
		}
		query.Set("t", "search")
		query.Set("q", title)
		mode = "search-query"
	default:
		return ReleasePage{}, ErrUnsupported
	}
	if category != 0 {
		query.Set("cat", strconv.Itoa(category))
	}
	data, err := s.client.request(ctx, config, query)
	if err != nil {
		return ReleasePage{}, err
	}
	page, err := parseReleases(data, config, limit, offset)
	if err != nil {
		return ReleasePage{}, err
	}
	current, err := s.repo.GetSource(ctx)
	if err != nil {
		return ReleasePage{}, err
	}
	if current.Revision != config.Revision {
		return ReleasePage{}, ErrChanged
	}
	page.MovieID = movie.ID
	page.SourceName = config.Name
	page.SourceRevision = config.Revision
	page.ProfileRevision = profile.Revision
	page.Mode = mode
	for i := range page.Items {
		explain(&page.Items[i], profile.Profile, movie.OriginalLanguage)
	}
	return page, nil
}

func explain(candidate *Candidate, profile settings.Profile, original string) {
	// Metadata can carry an absent or unrecognized language code too.
	if languages := audioLanguages([]string{original}); len(languages) == 1 {
		original = languages[0]
	} else {
		original = ""
	}
	candidate.Status = "match"
	candidate.Checks = []ProfileCheck{}
	check := func(dimension, status string, accepted []string) {
		candidate.Checks = append(candidate.Checks, ProfileCheck{Dimension: dimension, Status: status, Accepted: append([]string{}, accepted...)})
		if status == "mismatch" || candidate.Status == "match" && status == "unknown" {
			candidate.Status = status
		}
	}
	attribute := func(dimension string, value *string, accepted []string) {
		status := "unknown"
		if value != nil {
			status = "mismatch"
			if slices.Contains(accepted, *value) {
				status = "match"
			}
		}
		check(dimension, status, accepted)
	}
	attribute("resolution", candidate.Resolution, profile.AcceptedResolutions)
	attribute("format", candidate.Format, profile.AcceptedFormats)
	attribute("container", candidate.Container, profile.AcceptedContainers)
	candidate.PreferredResolution = candidate.Resolution != nil && *candidate.Resolution == profile.PreferredResolution
	status := "mismatch"
	if len(candidate.AudioLanguages) == 0 {
		status = "unknown"
	} else {
		for i, choice := range profile.AudioPreferences {
			language := choice
			if choice == "original" {
				language = original
				if original == "" {
					status = "unknown"
					continue
				}
			}
			if slices.Contains(candidate.AudioLanguages, language) {
				rank := i + 1
				candidate.AudioPreferenceRank = &rank
				status = "match"
				break
			}
		}
	}
	check("audio", status, profile.AudioPreferences)
}
