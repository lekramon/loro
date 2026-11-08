// Package storage owns SQLite connections, migrations, and the instance lock.
package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct {
	multiSourceSchema  bool
	nativeSourceSchema bool
	downloadSchema     bool
	submissionSchema   bool
	acquisitionSchema  bool
	catalogSchema      bool
	inventorySchema    bool
	reviewSchema       bool
	sourceSchema       bool
	clientSchema       bool
	write              *sql.DB
	read               *sql.DB
	lock               *flock.Flock
	path               string
}

func Open(ctx context.Context, directory string) (*Store, error) {
	return open(ctx, directory, migrations)
}

func open(ctx context.Context, directory string, source fs.FS) (_ *Store, resultErr error) {
	lock := flock.New(filepath.Join(directory, "piratarr.lock"))
	acquired, err := lock.TryLock()
	if err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("acquire instance lock: %w", err)
	}
	if !acquired {
		_ = lock.Close()
		return nil, errors.New("another instance is using this data directory")
	}
	s := &Store{lock: lock, path: filepath.Join(directory, "piratarr.db")}
	if _, err := fs.Stat(source, "migrations/00012_multiple_sources.sql"); err == nil {
		s.multiSourceSchema = true
	}
	if _, err := fs.Stat(source, "migrations/00011_native_source.sql"); err == nil {
		s.nativeSourceSchema = true
	}
	if _, err := fs.Stat(source, "migrations/00010_download_tracking.sql"); err == nil {
		s.downloadSchema = true
	}
	if _, err := fs.Stat(source, "migrations/00009_submission_jobs.sql"); err == nil {
		s.submissionSchema = true
	}
	if _, err := fs.Stat(source, "migrations/00008_acquisitions.sql"); err == nil {
		s.acquisitionSchema = true
	}
	if _, err := fs.Stat(source, "migrations/00007_download_client.sql"); err == nil {
		s.clientSchema = true
	}
	if _, err := fs.Stat(source, "migrations/00006_source.sql"); err == nil {
		s.sourceSchema = true
	}
	if _, err := fs.Stat(source, "migrations/00005_file_review.sql"); err == nil {
		s.reviewSchema = true
	}
	if _, err := fs.Stat(source, "migrations/00004_library_inventory.sql"); err == nil {
		s.inventorySchema = true
	}
	if _, err := fs.Stat(source, "migrations/00003_catalog.sql"); err == nil {
		s.catalogSchema = true
	}
	defer func() {
		if resultErr != nil {
			_ = s.Close()
		}
	}()
	// Create privately before SQLite opens it; WAL and SHM inherit directory ACLs
	// on Windows and SQLite's database permissions on Unix.
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open database file: %w", err)
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	if err = os.Chmod(s.path, 0600); err != nil {
		return nil, err
	}
	uri := url.URL{Scheme: "file", Path: filepath.ToSlash(s.path)}
	if filepath.VolumeName(s.path) != "" {
		uri.Path = "/" + uri.Path
	}
	q := url.Values{"_pragma": {"foreign_keys(1)", "busy_timeout(5000)"}}
	uri.RawQuery = q.Encode()
	s.write, err = sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	s.write.SetMaxOpenConns(1)
	if err = s.write.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	migrationFS, err := fs.Sub(source, "migrations")
	if err != nil {
		return nil, err
	}
	if err = validateHistory(ctx, s.write, migrationFS); err != nil {
		return nil, err
	}
	if _, err = s.write.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		return nil, err
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, s.write, migrationFS, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		return nil, fmt.Errorf("prepare migrations: %w", err)
	}
	if _, err = provider.Up(ctx); err != nil {
		return nil, fmt.Errorf("apply migrations: %w", err)
	}
	if s.acquisitionSchema {
		if _, err = s.write.ExecContext(ctx, "UPDATE acquisitions SET state='needs-review',revision=revision+1,review_code='preparation-interrupted',updated_at=? WHERE state='preparing'", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return nil, errors.New("recover acquisition preparation")
		}
	}
	if s.submissionSchema {
		if _, err = s.write.ExecContext(ctx, "UPDATE submission_jobs SET state='queued',lease_token=NULL,lease_expires=NULL WHERE state='running'"); err != nil {
			return nil, errors.New("recover submission jobs")
		}
	}
	if s.downloadSchema {
		if _, err = s.write.ExecContext(ctx, "UPDATE download_tasks SET poll_state='queued',lease_token=NULL,lease_expires=NULL WHERE poll_state='running'"); err != nil {
			return nil, errors.New("recover download checks")
		}
	}
	s.read, err = sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	s.read.SetMaxOpenConns(4)
	if err = s.read.PingContext(ctx); err != nil {
		return nil, err
	}
	if err = s.Ready(ctx); err != nil {
		return nil, fmt.Errorf("validate database schema: %w", err)
	}
	return s, nil
}

func validateHistory(ctx context.Context, db *sql.DB, source fs.FS) error {
	known := map[int64]bool{0: true}
	entries, err := fs.ReadDir(source, ".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(entry.Name(), "_")
		version, parseErr := strconv.ParseInt(prefix, 10, 64)
		if !ok || parseErr != nil || version < 1 || known[version] {
			return errors.New("invalid embedded migration sequence")
		}
		known[version] = true
	}
	var exists int
	if err = db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='goose_db_version'").Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return nil
	}
	rows, err := db.QueryContext(ctx, "SELECT version_id, is_applied FROM goose_db_version ORDER BY id")
	if err != nil {
		return fmt.Errorf("read migration history: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var version int64
		var applied bool
		if err = rows.Scan(&version, &applied); err != nil {
			return err
		}
		// Reject unknown entries even when marked down: this binary cannot
		// establish compatibility with a history it does not recognize.
		if !known[version] {
			return fmt.Errorf("unsupported database migration version %d", version)
		}
	}
	return rows.Err()
}

func (s *Store) Ready(ctx context.Context) error {
	// Ping may return a pooled connection even after directory access is lost.
	info, err := os.Stat(s.path)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("database file unavailable")
	}
	var value int
	query := "SELECT (SELECT count(*) FROM settings) + (SELECT count(*) FROM accepted_resolutions) + (SELECT count(*) FROM audio_preferences)"
	if s.catalogSchema {
		query += " + (SELECT count(*) FROM movies) + (SELECT count(*) FROM metadata_cache) + (SELECT count(*) FROM metadata_artwork)"
	}
	if s.inventorySchema {
		query += " + (SELECT count(*) FROM library_files) + (SELECT count(*) FROM library_scan_summary)"
	}
	if s.reviewSchema {
		query += " + (SELECT count(review_state) FROM library_files)"
	}
	if s.sourceSchema {
		query += " + (SELECT count(*) FROM source_configuration)"
	}
	if s.nativeSourceSchema {
		query += " + (SELECT count(kind) FROM source_configuration)"
	}
	if s.multiSourceSchema {
		query += " + (SELECT count(active)+count(id) FROM source_configuration)"
	}
	if s.clientSchema {
		query += " + (SELECT count(revision)+count(endpoint)+count(username)+count(password)+count(app_version)+count(api_version)+count(checked_at) FROM download_client_configuration)"
	}
	if s.acquisitionSchema {
		query += " + (SELECT count(snapshot)+count(reference) FROM release_observations) + (SELECT count(state)+count(revision)+count(payload) FROM acquisitions) + (SELECT count(request_key) FROM acquisition_requests)"
	}
	if s.submissionSchema {
		query += " + (SELECT count(kind)+count(payload_version)+count(client_revision)+count(settings_revision)+count(download_directory)+count(state)+count(due_at)+count(attempts)+count(lease_token)+count(lease_expires)+count(add_attempted)+count(error_code)+count(updated_at)+count(info_hash)+count(acquisition_id) FROM submission_jobs)"
	}
	if s.downloadSchema {
		query += " + (SELECT count(acquisition_id)+count(kind)+count(payload_version)+count(revision)+count(state)+count(snapshot)+count(error_code)+count(checked_at)+count(observed_at)+count(completed_at)+count(handoff)+count(poll_state)+count(due_at)+count(attempts)+count(lease_token)+count(lease_expires)+count(updated_at) FROM download_tasks) + (SELECT count(id)+count(acquisition_id)+count(state)+count(code)+count(progress)+count(handoff_state)+count(created_at) FROM download_events)"
	}
	return s.read.QueryRowContext(ctx, query).Scan(&value)
}

func (s *Store) SetupCompleted(ctx context.Context) (bool, error) {
	var count int
	err := s.read.QueryRowContext(ctx, "SELECT count(*) FROM settings WHERE id=1").Scan(&count)
	return count == 1, err
}

func (s *Store) Close() error {
	var errs []error
	if s.read != nil {
		errs = append(errs, s.read.Close())
	}
	if s.write != nil {
		errs = append(errs, s.write.Close())
	}
	if s.lock != nil {
		errs = append(errs, s.lock.Close())
	}
	return errors.Join(errs...)
}
