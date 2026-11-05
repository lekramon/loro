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
	mark := "UPDATE library_files SET present=0 WHERE root_key=?"
	if s.reviewSchema {
		mark = "UPDATE library_files SET present=0,revision=revision+1 WHERE root_key=?"
	}
	if _, err = tx.ExecContext(ctx, mark, key); err != nil {
		return library.ErrUnavailable
	}
	upsert := `INSERT INTO library_files(root_key,path_key,relative_path,size,modified_at,present,first_seen_at,last_seen_at) VALUES(?,?,?,?,?,1,?,?) ON CONFLICT(root_key,path_key) DO UPDATE SET relative_path=excluded.relative_path,size=excluded.size,modified_at=excluded.modified_at,present=1,last_seen_at=excluded.last_seen_at`
	if s.reviewSchema {
		upsert += `,review_state=CASE WHEN review_state='associated' AND (size<>excluded.size OR modified_at<>excluded.modified_at) THEN 'pending' ELSE review_state END,movie_id=CASE WHEN size<>excluded.size OR modified_at<>excluded.modified_at THEN NULL ELSE movie_id END,verified_at=CASE WHEN size<>excluded.size OR modified_at<>excluded.modified_at THEN NULL ELSE verified_at END`
	}
	statement, err := tx.PrepareContext(ctx, upsert)
	if err != nil {
		return library.ErrUnavailable
	}
	defer statement.Close()
	for _, file := range files {
		if _, err = statement.ExecContext(ctx, key, library.PathKey(file.RelativePath), file.RelativePath, file.Size, file.ModifiedAt, now, now); err != nil {
			return library.ErrUnavailable
		}
	}
	if s.reviewSchema {
		if _, err = tx.ExecContext(ctx, "UPDATE library_files SET review_state='pending',movie_id=NULL,verified_at=NULL WHERE root_key=? AND present=0 AND review_state='associated'", key); err != nil {
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
	rows, err := tx.QueryContext(ctx, "SELECT "+s.inventoryColumns()+" FROM library_files"+filter+" ORDER BY relative_path COLLATE NOCASE,id LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		return result, library.ErrUnavailable
	}
	for rows.Next() {
		entry, err := readInventory(rows)
		if err != nil {
			rows.Close()
			return result, library.ErrUnavailable
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

func (s *Store) inventoryColumns() string {
	base := "id,relative_path,size,modified_at,present,first_seen_at,last_seen_at"
	if !s.reviewSchema {
		return base + ",1,'pending',NULL,'',NULL"
	}
	return base + ",revision,review_state,movie_id,COALESCE((SELECT title FROM movies WHERE movies.id=library_files.movie_id),''),verified_at"
}
func readInventory(row scanner) (library.Entry, error) {
	var entry library.Entry
	var present bool
	err := row.Scan(&entry.ID, &entry.RelativePath, &entry.Size, &entry.ModifiedAt, &present, &entry.FirstSeenAt, &entry.LastSeenAt, &entry.Revision, &entry.ReviewState, &entry.MovieID, &entry.MovieTitle, &entry.VerifiedAt)
	entry.Status = "missing"
	if present {
		entry.Status = "present"
	}
	library.Suggest(&entry)
	return entry, err
}
func (s *Store) InventoryFile(ctx context.Context, root string, id int64) (library.Entry, error) {
	entry, err := readInventory(s.read.QueryRowContext(ctx, "SELECT "+s.inventoryColumns()+" FROM library_files WHERE root_key=? AND id=?", library.PathKey(root), id))
	if errors.Is(err, sql.ErrNoRows) {
		return entry, library.ErrNotFound
	}
	if err != nil {
		return entry, library.ErrUnavailable
	}
	return entry, nil
}
func (s *Store) CommitReview(ctx context.Context, root string, settingsRevision int, id int64, input library.Review) (library.Entry, error) {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return library.Entry{}, library.ErrUnavailable
	}
	defer tx.Rollback()
	var revision int
	var path string
	if err = tx.QueryRowContext(ctx, "SELECT revision,movie_directory FROM settings WHERE id=1").Scan(&revision, &path); err != nil {
		return library.Entry{}, library.ErrUnavailable
	}
	if revision != settingsRevision || library.PathKey(root) != library.PathKey(path) {
		return library.Entry{}, library.ErrChanged
	}
	entry, err := readInventory(tx.QueryRowContext(ctx, "SELECT "+s.inventoryColumns()+" FROM library_files WHERE root_key=? AND id=?", library.PathKey(root), id))
	if errors.Is(err, sql.ErrNoRows) {
		return entry, library.ErrNotFound
	}
	if err != nil {
		return entry, library.ErrUnavailable
	}
	if entry.Revision != input.Revision {
		return entry, library.ErrConflict
	}
	state := "pending"
	var movieID *int64
	var verified *string
	switch input.Action {
	case "associate":
		if entry.Status != "present" || entry.ReviewState == "ignored" {
			return entry, library.ErrConflict
		}
		var exists int
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM movies WHERE id=?", input.MovieID).Scan(&exists); err != nil {
			return entry, library.ErrUnavailable
		}
		if exists != 1 {
			return entry, library.ErrNotFound
		}
		state = "associated"
		movieID = input.MovieID
		at := time.Now().UTC().Format(time.RFC3339Nano)
		verified = &at
	case "ignore":
		state = "ignored"
	case "reopen":
		if entry.ReviewState != "ignored" {
			return entry, library.ErrConflict
		}
	default:
		return entry, library.ErrConflict
	}
	if _, err = tx.ExecContext(ctx, "UPDATE library_files SET revision=revision+1,review_state=?,movie_id=?,verified_at=? WHERE id=? AND revision=?", state, movieID, verified, id, input.Revision); err != nil {
		return entry, library.ErrUnavailable
	}
	entry, err = readInventory(tx.QueryRowContext(ctx, "SELECT "+s.inventoryColumns()+" FROM library_files WHERE id=?", id))
	if err != nil {
		return entry, library.ErrUnavailable
	}
	if err = tx.Commit(); err != nil {
		return entry, library.ErrUnavailable
	}
	return entry, nil
}
