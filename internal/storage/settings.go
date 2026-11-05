package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"time"

	"github.com/lekramon/piratarr/internal/settings"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func (s *Store) DataDirectory() string { return filepath.Dir(s.path) }

func databaseError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return settings.ErrUnavailable
	}
	var failure *sqlite.Error
	if errors.As(err, &failure) {
		switch failure.Code() & 0xff {
		case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED, sqlite3.SQLITE_IOERR, sqlite3.SQLITE_CANTOPEN, sqlite3.SQLITE_READONLY, sqlite3.SQLITE_FULL:
			return settings.ErrUnavailable
		}
	}
	return err
}

func (s *Store) CreateSettings(ctx context.Context, input settings.Input) (settings.Settings, error) {
	if s.Ready(ctx) != nil {
		return settings.Settings{}, settings.ErrUnavailable
	}
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return settings.Settings{}, databaseError(err)
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM settings WHERE id=1").Scan(&count); err != nil {
		return settings.Settings{}, databaseError(err)
	}
	if count > 0 {
		return settings.Settings{}, settings.ErrAlreadyCompleted
	}
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, "INSERT INTO settings(id,revision,download_directory,movie_directory,preferred_resolution,created_at,updated_at) VALUES(1,1,?,?,?,?,?)", input.Directories.DownloadDirectory, input.Directories.MovieDirectory, input.Profile.PreferredResolution, timestamp, timestamp)
	if err != nil {
		return settings.Settings{}, databaseError(err)
	}
	for _, resolution := range input.Profile.AcceptedResolutions {
		if _, err = tx.ExecContext(ctx, "INSERT INTO accepted_resolutions(settings_id,resolution) VALUES(1,?)", resolution); err != nil {
			return settings.Settings{}, databaseError(err)
		}
	}
	for position, choice := range input.Profile.AudioPreferences {
		if _, err = tx.ExecContext(ctx, "INSERT INTO audio_preferences(settings_id,position,choice) VALUES(1,?,?)", position, choice); err != nil {
			return settings.Settings{}, databaseError(err)
		}
	}
	if err = tx.Commit(); err != nil {
		return settings.Settings{}, databaseError(err)
	}
	return settings.Settings{Revision: 1, Directories: input.Directories, Profile: input.Profile, UpdatedAt: timestamp}, nil
}

func (s *Store) GetSettings(ctx context.Context) (settings.Settings, error) {
	result := settings.Settings{}
	if s.Ready(ctx) != nil {
		return result, settings.ErrUnavailable
	}
	tx, err := s.read.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return result, databaseError(err)
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, "SELECT revision,download_directory,movie_directory,preferred_resolution,updated_at FROM settings WHERE id=1").Scan(&result.Revision, &result.Directories.DownloadDirectory, &result.Directories.MovieDirectory, &result.Profile.PreferredResolution, &result.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return result, settings.ErrSetupRequired
	}
	if err != nil {
		return result, databaseError(err)
	}
	rows, err := tx.QueryContext(ctx, "SELECT resolution FROM accepted_resolutions WHERE settings_id=1 ORDER BY CASE resolution WHEN '480p' THEN 1 WHEN '576p' THEN 2 WHEN '720p' THEN 3 WHEN '1080p' THEN 4 WHEN '2160p' THEN 5 END")
	if err != nil {
		return result, databaseError(err)
	}
	result.Profile.AcceptedResolutions = []string{}
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			_ = rows.Close()
			return result, databaseError(err)
		}
		result.Profile.AcceptedResolutions = append(result.Profile.AcceptedResolutions, value)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return result, databaseError(err)
	}
	rows, err = tx.QueryContext(ctx, "SELECT choice FROM audio_preferences WHERE settings_id=1 ORDER BY position")
	if err != nil {
		return result, databaseError(err)
	}
	result.Profile.AudioPreferences = []string{}
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			_ = rows.Close()
			return result, databaseError(err)
		}
		result.Profile.AudioPreferences = append(result.Profile.AudioPreferences, value)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return result, databaseError(err)
	}
	if err = tx.Commit(); err != nil {
		return result, databaseError(err)
	}
	return result, nil
}

func (s *Store) UpdateSettings(ctx context.Context, revision int, input settings.Input) (settings.Settings, error) {
	if s.Ready(ctx) != nil {
		return settings.Settings{}, settings.ErrUnavailable
	}
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return settings.Settings{}, databaseError(err)
	}
	defer tx.Rollback()
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, "UPDATE settings SET revision=revision+1, download_directory=?, movie_directory=?, preferred_resolution=?, updated_at=? WHERE id=1 AND revision=?", input.Directories.DownloadDirectory, input.Directories.MovieDirectory, input.Profile.PreferredResolution, timestamp, revision)
	if err != nil {
		return settings.Settings{}, databaseError(err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return settings.Settings{}, databaseError(err)
	}
	if changed == 0 {
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM settings WHERE id=1").Scan(&count); err != nil {
			return settings.Settings{}, databaseError(err)
		}
		if count == 0 {
			return settings.Settings{}, settings.ErrSetupRequired
		}
		return settings.Settings{}, settings.ErrRevisionConflict
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM accepted_resolutions WHERE settings_id=1"); err != nil {
		return settings.Settings{}, databaseError(err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM audio_preferences WHERE settings_id=1"); err != nil {
		return settings.Settings{}, databaseError(err)
	}
	for _, resolution := range input.Profile.AcceptedResolutions {
		if _, err = tx.ExecContext(ctx, "INSERT INTO accepted_resolutions(settings_id,resolution) VALUES(1,?)", resolution); err != nil {
			return settings.Settings{}, databaseError(err)
		}
	}
	for position, choice := range input.Profile.AudioPreferences {
		if _, err = tx.ExecContext(ctx, "INSERT INTO audio_preferences(settings_id,position,choice) VALUES(1,?,?)", position, choice); err != nil {
			return settings.Settings{}, databaseError(err)
		}
	}
	if err = tx.Commit(); err != nil {
		return settings.Settings{}, databaseError(err)
	}
	return settings.Settings{Revision: revision + 1, Directories: input.Directories, Profile: input.Profile, UpdatedAt: timestamp}, nil
}
