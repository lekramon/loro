package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/lekramon/piratarr/internal/series"
)

func (s *Store) CreateSeries(ctx context.Context, input series.Input) (series.Series, error) {
	if input.Provider == "" {
		input.Provider = "tmdb"
	}
	if input.Provider != "tmdb" || input.ProviderID < 1 || strings.TrimSpace(input.Title) == "" {
		return series.Series{}, series.ErrUnavailable
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.write.ExecContext(ctx, "INSERT INTO series(provider,provider_id,title,metadata,anime,added_at) VALUES(?,?,?,'{}',?,?)", input.Provider, input.ProviderID, strings.TrimSpace(input.Title), boolInt(input.Anime), now)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return series.Series{}, series.ErrConflict
		}
		return series.Series{}, series.ErrUnavailable
	}
	id, err := result.LastInsertId()
	if err != nil {
		return series.Series{}, series.ErrUnavailable
	}
	return series.Series{ID: id, Provider: input.Provider, ProviderID: input.ProviderID, Title: strings.TrimSpace(input.Title), Anime: input.Anime, Metadata: "{}", AddedAt: now}, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (s *Store) ListSeries(ctx context.Context, limit, offset int, anime *bool) (series.Page[series.Series], error) {
	page := series.Page[series.Series]{Items: []series.Series{}, Limit: limit, Offset: offset}
	where, args := "", []any{}
	if anime != nil {
		where = " WHERE anime=?"
		args = append(args, boolInt(*anime))
	}
	if err := s.read.QueryRowContext(ctx, "SELECT count(*) FROM series"+where, args...).Scan(&page.Total); err != nil {
		return page, series.ErrUnavailable
	}
	args = append(args, limit, offset)
	rows, err := s.read.QueryContext(ctx, "SELECT id,provider,provider_id,title,anime,metadata,added_at FROM series"+where+" ORDER BY title COLLATE NOCASE,id LIMIT ? OFFSET ?", args...)
	if err != nil {
		return page, series.ErrUnavailable
	}
	defer rows.Close()
	for rows.Next() {
		var item series.Series
		var animeValue int
		if err = rows.Scan(&item.ID, &item.Provider, &item.ProviderID, &item.Title, &animeValue, &item.Metadata, &item.AddedAt); err != nil {
			return page, series.ErrUnavailable
		}
		item.Anime = animeValue == 1
		page.Items = append(page.Items, item)
	}
	return page, rows.Err()
}

func (s *Store) GetSeries(ctx context.Context, id int64) (series.Series, error) {
	var item series.Series
	var animeValue int
	err := s.read.QueryRowContext(ctx, "SELECT id,provider,provider_id,title,anime,metadata,added_at FROM series WHERE id=?", id).Scan(&item.ID, &item.Provider, &item.ProviderID, &item.Title, &animeValue, &item.Metadata, &item.AddedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return item, series.ErrNotFound
	}
	if err != nil {
		return item, series.ErrUnavailable
	}
	item.Anime = animeValue == 1
	return item, nil
}

func (s *Store) ListSeasons(ctx context.Context, seriesID int64) ([]series.Season, error) {
	rows, err := s.read.QueryContext(ctx, "SELECT id,series_id,season_number,title,metadata FROM seasons WHERE series_id=? ORDER BY season_number,id", seriesID)
	if err != nil {
		return nil, series.ErrUnavailable
	}
	defer rows.Close()
	items := []series.Season{}
	for rows.Next() {
		var item series.Season
		if err = rows.Scan(&item.ID, &item.SeriesID, &item.SeasonNumber, &item.Title, &item.Metadata); err != nil {
			return nil, series.ErrUnavailable
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListEpisodes(ctx context.Context, seriesID int64, limit, offset int, season *int) (series.Page[series.Episode], error) {
	page := series.Page[series.Episode]{Items: []series.Episode{}, Limit: limit, Offset: offset}
	where := " WHERE s.series_id=?"
	args := []any{seriesID}
	if season != nil {
		where += " AND s.season_number=?"
		args = append(args, *season)
	}
	if err := s.read.QueryRowContext(ctx, "SELECT count(*) FROM episodes e JOIN seasons s ON s.id=e.season_id"+where, args...).Scan(&page.Total); err != nil {
		return page, series.ErrUnavailable
	}
	args = append(args, limit, offset)
	rows, err := s.read.QueryContext(ctx, "SELECT e.id,e.season_id,e.provider_id,e.episode_number,e.absolute_number,e.title,e.metadata FROM episodes e JOIN seasons s ON s.id=e.season_id"+where+" ORDER BY s.season_number,e.episode_number,e.id LIMIT ? OFFSET ?", args...)
	if err != nil {
		return page, series.ErrUnavailable
	}
	defer rows.Close()
	for rows.Next() {
		var item series.Episode
		if err = rows.Scan(&item.ID, &item.SeasonID, &item.ProviderID, &item.EpisodeNumber, &item.AbsoluteNumber, &item.Title, &item.Metadata); err != nil {
			return page, series.ErrUnavailable
		}
		page.Items = append(page.Items, item)
	}
	return page, rows.Err()
}

func (s *Store) GetEpisode(ctx context.Context, id int64) (series.Episode, error) {
	var item series.Episode
	err := s.read.QueryRowContext(ctx, "SELECT id,season_id,provider_id,episode_number,absolute_number,title,metadata FROM episodes WHERE id=?", id).Scan(&item.ID, &item.SeasonID, &item.ProviderID, &item.EpisodeNumber, &item.AbsoluteNumber, &item.Title, &item.Metadata)
	if errors.Is(err, sql.ErrNoRows) {
		return item, series.ErrNotFound
	}
	if err != nil {
		return item, series.ErrUnavailable
	}
	return item, nil
}
