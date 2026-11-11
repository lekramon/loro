// Package series owns series, season and episode catalog identities.
package series

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("series entity not found")
var ErrConflict = errors.New("series identity already exists")
var ErrUnavailable = errors.New("series storage unavailable")

type Series struct {
	ID         int64  `json:"id"`
	Provider   string `json:"provider"`
	ProviderID int64  `json:"providerId"`
	Title      string `json:"title"`
	Anime      bool   `json:"anime"`
	Metadata   string `json:"metadata"`
	AddedAt    string `json:"addedAt"`
}
type Season struct {
	ID           int64   `json:"id"`
	SeriesID     int64   `json:"seriesId"`
	SeasonNumber int     `json:"seasonNumber"`
	Title        *string `json:"title,omitempty"`
	Metadata     string  `json:"metadata"`
}
type Episode struct {
	ID             int64  `json:"id"`
	SeasonID       int64  `json:"seasonId"`
	ProviderID     *int64 `json:"providerId,omitempty"`
	EpisodeNumber  *int   `json:"episodeNumber,omitempty"`
	AbsoluteNumber *int   `json:"absoluteNumber,omitempty"`
	Title          string `json:"title"`
	Metadata       string `json:"metadata"`
}
type Page[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}
type Input struct {
	Provider   string `json:"provider"`
	ProviderID int64  `json:"providerId"`
	Title      string `json:"title"`
	Anime      bool   `json:"anime"`
}

type Repository interface {
	CreateSeries(context.Context, Input) (Series, error)
	ListSeries(context.Context, int, int, *bool) (Page[Series], error)
	GetSeries(context.Context, int64) (Series, error)
	ListSeasons(context.Context, int64) ([]Season, error)
	ListEpisodes(context.Context, int64, int, int, *int) (Page[Episode], error)
	GetEpisode(context.Context, int64) (Episode, error)
}
