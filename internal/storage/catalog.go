package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/library"
	"github.com/lekramon/piratarr/internal/settings"
)

func catalogError(err error) error {
	if errors.Is(databaseError(err), settings.ErrUnavailable) {
		return catalog.ErrUnavailable
	}
	return err
}

type scanner interface{ Scan(...any) error }

func scanMovie(row scanner) (catalog.Movie, error) {
	var movie catalog.Movie
	var data []byte
	err := row.Scan(&movie.ID, &data, &movie.AddedAt, &movie.HasMedia)
	if errors.Is(err, sql.ErrNoRows) {
		return movie, catalog.ErrNotFound
	}
	if err != nil {
		return movie, catalogError(err)
	}
	if err = json.Unmarshal(data, &movie.Metadata); err != nil {
		return movie, err
	}
	return movie, nil
}
func (s *Store) MovieByTMDB(ctx context.Context, id int64) (catalog.Movie, error) {
	root, err := movieRoot(ctx, s.read)
	if err != nil {
		return catalog.Movie{}, err
	}
	return scanMovie(s.read.QueryRowContext(ctx, "SELECT "+s.movieColumns()+" FROM movies WHERE tmdb_id=?", root, id))
}
func (s *Store) MovieByID(ctx context.Context, id int64) (catalog.Movie, error) {
	root, err := movieRoot(ctx, s.read)
	if err != nil {
		return catalog.Movie{}, err
	}
	return scanMovie(s.read.QueryRowContext(ctx, "SELECT "+s.movieColumns()+" FROM movies WHERE id=?", root, id))
}
func (s *Store) RegisterMovie(ctx context.Context, metadata catalog.Metadata) (catalog.Movie, bool, error) {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return catalog.Movie{}, false, err
	}
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return catalog.Movie{}, false, catalogError(err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "INSERT INTO movies(tmdb_id,title,metadata,added_at) VALUES(?,?,?,?) ON CONFLICT(tmdb_id) DO NOTHING", metadata.TMDBID, metadata.Title, string(payload), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return catalog.Movie{}, false, catalogError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return catalog.Movie{}, false, catalogError(err)
	}
	root, err := movieRoot(ctx, tx)
	if err != nil {
		return catalog.Movie{}, false, err
	}
	movie, err := scanMovie(tx.QueryRowContext(ctx, "SELECT "+s.movieColumns()+" FROM movies WHERE tmdb_id=?", root, metadata.TMDBID))
	if err != nil {
		return movie, false, err
	}
	if err = tx.Commit(); err != nil {
		return movie, false, catalogError(err)
	}
	return movie, affected == 1, nil
}
func (s *Store) ListMovies(ctx context.Context, limit, offset int, query string) (catalog.ListResult, error) {
	result := catalog.ListResult{Items: []catalog.Movie{}, Limit: limit, Offset: offset}
	tx, err := s.read.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return result, catalogError(err)
	}
	defer tx.Rollback()
	pattern := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(query) + "%"
	const filter = ` WHERE title LIKE ? ESCAPE '\' OR json_extract(metadata,'$.originalTitle') LIKE ? ESCAPE '\'`
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM movies"+filter, pattern, pattern).Scan(&result.Total); err != nil {
		return result, catalogError(err)
	}
	root, err := movieRoot(ctx, tx)
	if err != nil {
		return result, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT "+s.movieColumns()+" FROM movies"+filter+" ORDER BY title COLLATE NOCASE,id LIMIT ? OFFSET ?", root, pattern, pattern, limit, offset)
	if err != nil {
		return result, catalogError(err)
	}
	for rows.Next() {
		movie, e := scanMovie(rows)
		if e != nil {
			rows.Close()
			return result, e
		}
		result.Items = append(result.Items, movie)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, catalogError(err)
	}
	if err = tx.Commit(); err != nil {
		return result, catalogError(err)
	}
	return result, nil
}
func (s *Store) CachedMetadata(ctx context.Context, key string) ([]byte, time.Time, error) {
	var payload []byte
	var timestamp string
	err := s.read.QueryRowContext(ctx, "SELECT payload,fetched_at FROM metadata_cache WHERE cache_key=?", key).Scan(&payload, &timestamp)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, time.Time{}, catalog.ErrNotFound
	}
	if err != nil {
		return nil, time.Time{}, catalogError(err)
	}
	at, err := time.Parse(time.RFC3339Nano, timestamp)
	return payload, at, err
}
func (s *Store) CacheMetadata(ctx context.Context, key string, payload []byte, at time.Time, artwork []string) error {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return catalogError(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "INSERT INTO metadata_cache(cache_key,payload,fetched_at) VALUES(?,?,?) ON CONFLICT(cache_key) DO UPDATE SET payload=excluded.payload,fetched_at=excluded.fetched_at", key, payload, at.UTC().Format(time.RFC3339Nano)); err != nil {
		return catalogError(err)
	}
	for _, path := range artwork {
		if _, err = tx.ExecContext(ctx, "INSERT INTO metadata_artwork(local_url) VALUES(?) ON CONFLICT DO NOTHING", path); err != nil {
			return catalogError(err)
		}
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM metadata_cache WHERE cache_key IN (SELECT cache_key FROM metadata_cache ORDER BY fetched_at DESC,cache_key LIMIT -1 OFFSET 500)"); err != nil {
		return catalogError(err)
	}
	return catalogError(tx.Commit())
}
func (s *Store) KnownArtwork(ctx context.Context, path string) (bool, error) {
	var count int
	err := s.read.QueryRowContext(ctx, "SELECT count(*) FROM metadata_artwork WHERE local_url=?", path).Scan(&count)
	return count > 0, catalogError(err)
}

func (s *Store) movieColumns() string {
	base := "id,metadata,added_at"
	if !s.reviewSchema {
		return base + ",0*length(?)"
	}
	return base + ",EXISTS(SELECT 1 FROM library_files WHERE library_files.movie_id=movies.id AND present=1 AND review_state='associated' AND root_key=?)"
}

type movieQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func movieRoot(ctx context.Context, db movieQueryer) (string, error) {
	var root string
	err := db.QueryRowContext(ctx, "SELECT movie_directory FROM settings WHERE id=1").Scan(&root)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", catalogError(err)
	}
	return library.PathKey(root), nil
}
