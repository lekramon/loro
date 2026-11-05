// Package catalog owns movie identity, offline records, and metadata caching.
package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const Attribution = "This product uses the TMDB API but is not endorsed or certified by TMDB."
const ImagePrefix = "/api/v1/metadata/images/"

type Metadata struct {
	TMDBID           int64    `json:"tmdbId"`
	Title            string   `json:"title"`
	OriginalTitle    string   `json:"originalTitle"`
	Year             *int     `json:"year"`
	OriginalLanguage string   `json:"originalLanguage"`
	Overview         string   `json:"overview"`
	RuntimeMinutes   *int     `json:"runtimeMinutes"`
	Genres           []string `json:"genres"`
	PosterURL        string   `json:"posterUrl"`
	BackdropURL      string   `json:"backdropUrl"`
	MetadataLanguage string   `json:"metadataLanguage"`
}
type Movie struct {
	ID int64 `json:"id"`
	Metadata
	AddedAt  string `json:"addedAt"`
	HasMedia bool   `json:"hasMedia"`
}
type CacheState struct {
	Hit   bool `json:"hit"`
	Stale bool `json:"stale"`
}
type SearchResult struct {
	Page         int        `json:"page"`
	TotalPages   int        `json:"totalPages"`
	TotalResults int        `json:"totalResults"`
	Results      []Metadata `json:"results"`
	Cache        CacheState `json:"cache"`
	Warning      string     `json:"warning,omitempty"`
}
type ListResult struct {
	Items  []Movie `json:"items"`
	Total  int     `json:"total"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
}
type ProviderError struct {
	Code       string
	RetryAfter int
}

func (e *ProviderError) Error() string { return e.Code }

var ErrNotFound = errors.New("catalog movie not found")
var ErrUnavailable = errors.New("catalog storage unavailable")

type Provider interface {
	Configured() bool
	Search(context.Context, string, int, int, string) (SearchResult, error)
	Details(context.Context, int64, string) (Metadata, error)
	Image(context.Context, string, string) ([]byte, error)
}
type Repository interface {
	MovieByTMDB(context.Context, int64) (Movie, error)
	MovieByID(context.Context, int64) (Movie, error)
	RegisterMovie(context.Context, Metadata) (Movie, bool, error)
	ListMovies(context.Context, int, int, string) (ListResult, error)
	CachedMetadata(context.Context, string) ([]byte, time.Time, error)
	CacheMetadata(context.Context, string, []byte, time.Time, []string) error
	KnownArtwork(context.Context, string) (bool, error)
}
type Service struct {
	ImageCacheBytes int64
	Repo            Repository
	Provider        Provider
	DataDirectory   string
	Now             func() time.Time
	registrations   [32]chan struct{}
	imageGate       chan struct{}
}

func New(repo Repository, provider Provider, directory string) *Service {
	s := &Service{Repo: repo, Provider: provider, DataDirectory: directory, Now: time.Now, ImageCacheBytes: 128 << 20, imageGate: make(chan struct{}, 1)}
	for i := range s.registrations {
		s.registrations[i] = make(chan struct{}, 1)
	}
	return s
}
func Language(value string) (string, bool) {
	if value == "" {
		return "en-US", true
	}
	return value, value == "en-US" || value == "pt-BR"
}
func (s *Service) Search(ctx context.Context, query string, year, page int, language string) (SearchResult, error) {
	key := "search:" + url.QueryEscape(strings.Join(strings.Fields(query), " ")) + fmt.Sprintf(":%d:%d:%s", year, page, language)
	var previous SearchResult
	data, at, cacheErr := s.Repo.CachedMetadata(ctx, key)
	valid := cacheErr == nil && json.Unmarshal(data, &previous) == nil
	if cacheErr != nil && !errors.Is(cacheErr, ErrNotFound) {
		return SearchResult{}, cacheErr
	}
	if valid && s.Now().Sub(at) < 24*time.Hour {
		previous.Cache = CacheState{Hit: true}
		return previous, nil
	}
	result, err := s.Provider.Search(ctx, query, year, page, language)
	if err != nil {
		if valid {
			previous.Cache = CacheState{Hit: true, Stale: true}
			var failure *ProviderError
			if errors.As(err, &failure) {
				previous.Warning = failure.Code
			} else {
				previous.Warning = "metadata-unavailable"
			}
			return previous, nil
		}
		return SearchResult{}, err
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return SearchResult{}, err
	}
	if err = s.Repo.CacheMetadata(ctx, key, payload, s.Now(), artwork(result.Results)); err != nil {
		return SearchResult{}, err
	}
	return result, nil
}
func (s *Service) Register(ctx context.Context, id int64, language string) (Movie, bool, error) {
	gate := s.registrations[id%int64(len(s.registrations))]
	select {
	case gate <- struct{}{}:
		defer func() { <-gate }()
	case <-ctx.Done():
		return Movie{}, false, ErrUnavailable
	}
	movie, err := s.Repo.MovieByTMDB(ctx, id)
	if err == nil {
		return movie, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Movie{}, false, err
	}
	key := fmt.Sprintf("movie:%d:%s", id, language)
	var metadata Metadata
	data, at, cacheErr := s.Repo.CachedMetadata(ctx, key)
	valid := cacheErr == nil && json.Unmarshal(data, &metadata) == nil && metadata.TMDBID == id
	if cacheErr != nil && !errors.Is(cacheErr, ErrNotFound) {
		return Movie{}, false, cacheErr
	}
	if !valid || s.Now().Sub(at) >= 24*time.Hour {
		metadata, err = s.Provider.Details(ctx, id, language)
		if err != nil {
			return Movie{}, false, err
		}
		payload, marshalErr := json.Marshal(metadata)
		if marshalErr != nil {
			return Movie{}, false, marshalErr
		}
		if err = s.Repo.CacheMetadata(ctx, key, payload, s.Now(), artwork([]Metadata{metadata})); err != nil {
			return Movie{}, false, err
		}
	}
	return s.Repo.RegisterMovie(ctx, metadata)
}
func artwork(movies []Metadata) []string {
	result := []string{}
	for _, movie := range movies {
		for _, path := range []string{movie.PosterURL, movie.BackdropURL} {
			if strings.HasPrefix(path, ImagePrefix) {
				result = append(result, path)
			}
		}
	}
	return result
}
