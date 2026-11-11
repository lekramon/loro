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
	ObservationID       string         `json:"observationId"`
	DownloadReference   string         `json:"-"`
	HashClaim           string         `json:"-"`
	ReferenceIssue      string         `json:"-"`
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
	SourceURL       string      `json:"-"`
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

type EpisodeSearch struct {
	EpisodeID      int64  `json:"episodeId"`
	Query          string `json:"query"`
	SeasonNumber   *int   `json:"seasonNumber,omitempty"`
	EpisodeNumber  *int   `json:"episodeNumber,omitempty"`
	AbsoluteNumber *int   `json:"absoluteNumber,omitempty"`
}

type EpisodeSearchResult struct {
	EpisodeID int64       `json:"episodeId"`
	Query     string      `json:"query"`
	Items     []Candidate `json:"items"`
}

func (s *Service) SearchEpisode(ctx context.Context, input EpisodeSearch) (EpisodeSearchResult, error) {
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	default:
		return EpisodeSearchResult{}, ErrBusy
	}
	input.Query = strings.TrimSpace(input.Query)
	if input.EpisodeID < 1 || input.Query == "" || len(input.Query) > 200 {
		return EpisodeSearchResult{}, ErrValidation
	}
	if input.SeasonNumber != nil && *input.SeasonNumber < 0 || input.EpisodeNumber != nil && *input.EpisodeNumber < 1 || input.AbsoluteNumber != nil && *input.AbsoluteNumber < 1 {
		return EpisodeSearchResult{}, ErrValidation
	}
	config, err := s.repo.GetSource(ctx)
	if err != nil {
		return EpisodeSearchResult{}, err
	}
	if !config.Configured {
		return EpisodeSearchResult{}, ErrUnconfigured
	}
	if config.Capabilities == nil {
		return EpisodeSearchResult{}, ErrCapabilities
	}
	if !config.Capabilities.Search.Available || !contains(config.Capabilities.Search.SupportedParams, "q") {
		return EpisodeSearchResult{}, ErrUnsupported
	}
	query := url.Values{"t": {"search"}, "q": {input.Query}, "extended": {"1"}, "limit": {"25"}, "offset": {"0"}}
	if input.SeasonNumber != nil {
		query.Set("season", strconv.Itoa(*input.SeasonNumber))
	}
	if input.EpisodeNumber != nil {
		query.Set("ep", strconv.Itoa(*input.EpisodeNumber))
	}
	if input.AbsoluteNumber != nil {
		query.Set("ep_abs", strconv.Itoa(*input.AbsoluteNumber))
	}
	var page ReleasePage
	if config.Kind == "1337x" {
		return EpisodeSearchResult{}, ErrUnsupported
	}
	data, err := s.client.request(ctx, config, query)
	if err != nil {
		return EpisodeSearchResult{}, err
	}
	page, err = parseReleases(data, config, 25, 0)
	if err != nil {
		return EpisodeSearchResult{}, err
	}
	current, err := s.repo.GetSource(ctx)
	if err != nil {
		return EpisodeSearchResult{}, err
	}
	if current.Revision != config.Revision {
		return EpisodeSearchResult{}, ErrChanged
	}
	return EpisodeSearchResult{EpisodeID: input.EpisodeID, Query: input.Query, Items: page.Items}, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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
	var page ReleasePage
	if config.Kind == "1337x" {
		page, err = search1337x(ctx, config, query, limit, offset)
		mode = "1337x-query"
	} else {
		var data []byte
		data, err = s.client.request(ctx, config, query)
		if err == nil {
			page, err = parseReleases(data, config, limit, offset)
		}
	}
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
	page.SourceURL = config.URL
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
