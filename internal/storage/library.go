package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lekramon/piratarr/internal/library"
)

func (s *Store) PublishInventory(ctx context.Context, root string, revision int, files []library.Observation, skipped int) error {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return library.ErrUnavailable
	}
	defer tx.Rollback()
	var current int
	var path string
	if err = tx.QueryRowContext(ctx, "SELECT revision,movie_directory FROM settings WHERE id=1").Scan(&current, &path); err != nil {
		return library.ErrUnavailable
	}
	if current != revision || library.PathKey(path) != library.PathKey(root) {
		return library.ErrChanged
	}
	key := library.PathKey(root)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, "UPDATE library_files SET present=0 WHERE root_key=?", key); err != nil {
		return library.ErrUnavailable
	}
	statement, err := tx.PrepareContext(ctx, `INSERT INTO library_files(root_key,path_key,relative_path,size,modified_at,present,first_seen_at,last_seen_at) VALUES(?,?,?,?,?,1,?,?) ON CONFLICT(root_key,path_key) DO UPDATE SET relative_path=excluded.relative_path,size=excluded.size,modified_at=excluded.modified_at,present=1,last_seen_at=excluded.last_seen_at`)
	if err != nil {
		return library.ErrUnavailable
	}
	defer statement.Close()
	for _, file := range files {
		if _, err = statement.ExecContext(ctx, key, library.PathKey(file.RelativePath), file.RelativePath, file.Size, file.ModifiedAt, now, now); err != nil {
			return library.ErrUnavailable
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO library_scan_summary VALUES(?,?,?,?) ON CONFLICT(root_key) DO UPDATE SET scanned_at=excluded.scanned_at,files_present=excluded.files_present,skipped_entries=excluded.skipped_entries`, key, now, len(files), skipped); err != nil {
		return library.ErrUnavailable
	}
	if err = tx.Commit(); err != nil {
		return library.ErrUnavailable
	}
	return nil
}
func (s *Store) ListInventory(ctx context.Context, root string, limit, offset int, status string) (library.Snapshot, error) {
	result := library.Snapshot{Root: root, Items: []library.Entry{}, Limit: limit, Offset: offset}
	tx, err := s.read.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return result, library.ErrUnavailable
	}
	defer tx.Rollback()
	key := library.PathKey(root)
	summary := &library.Summary{}
	err = tx.QueryRowContext(ctx, "SELECT scanned_at,files_present,skipped_entries FROM library_scan_summary WHERE root_key=?", key).Scan(&summary.ScannedAt, &summary.FilesPresent, &summary.SkippedEntries)
	if err == nil {
		result.Scan = summary
	} else if !errors.Is(err, sql.ErrNoRows) {
		return result, library.ErrUnavailable
	}
	filter := " WHERE root_key=?"
	args := []any{key}
	if status != "all" {
		filter += " AND present=?"
		args = append(args, status == "present")
	}
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM library_files"+filter, args...).Scan(&result.Total); err != nil {
		return result, library.ErrUnavailable
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,relative_path,size,modified_at,present,first_seen_at,last_seen_at FROM library_files"+filter+" ORDER BY relative_path COLLATE NOCASE,id LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		return result, library.ErrUnavailable
	}
	for rows.Next() {
		var entry library.Entry
		var present bool
		if err = rows.Scan(&entry.ID, &entry.RelativePath, &entry.Size, &entry.ModifiedAt, &present, &entry.FirstSeenAt, &entry.LastSeenAt); err != nil {
			rows.Close()
			return result, library.ErrUnavailable
		}
		entry.Status = "missing"
		if present {
			entry.Status = "present"
		}
		result.Items = append(result.Items, entry)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, library.ErrUnavailable
	}
	if err = tx.Commit(); err != nil {
		return result, library.ErrUnavailable
	}
	return result, nil
}
