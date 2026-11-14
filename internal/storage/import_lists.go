package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lekramon/piratarr/internal/importlists"
)

func (s *Store) SaveImportListPreview(ctx context.Context, preview importlists.Preview) error {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return importlists.ErrUnavailable
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO import_list_previews(id,source_kind,list_key,name,language,input_hash,state,revision,created_at) VALUES(?,?,?,?,?,?,'preview',1,?)`, preview.ID, preview.Source, preview.ListKey, preview.Name, preview.Language, preview.InputHash, preview.CreatedAt)
	if err != nil {
		return importlists.ErrUnavailable
	}
	for _, row := range preview.Rows {
		_, err = tx.ExecContext(ctx, `INSERT INTO import_list_preview_rows(preview_id,row_number,source_item_key,title,year,tmdb_id,status,error_code) VALUES(?,?,?,?,?,?,?,?)`, preview.ID, row.RowNumber, row.SourceItemKey, row.Title, row.Year, row.TMDBID, row.Status, nullable(row.ErrorCode))
		if err != nil {
			return importlists.ErrUnavailable
		}
	}
	if err = tx.Commit(); err != nil {
		return importlists.ErrUnavailable
	}
	return nil
}

func (s *Store) ImportListPreview(ctx context.Context, id string) (importlists.Preview, error) {
	var preview importlists.Preview
	err := s.read.QueryRowContext(ctx, `SELECT id,source_kind,list_key,name,language,input_hash,state,revision,created_at FROM import_list_previews WHERE id=?`, id).Scan(&preview.ID, &preview.Source, &preview.ListKey, &preview.Name, &preview.Language, &preview.InputHash, &preview.State, &preview.Revision, &preview.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return preview, importlists.ErrNotFound
	}
	if err != nil {
		return preview, importlists.ErrUnavailable
	}
	rows, err := s.read.QueryContext(ctx, `SELECT row_number,source_item_key,title,year,tmdb_id,status,COALESCE(error_code,'') FROM import_list_preview_rows WHERE preview_id=? ORDER BY row_number`, id)
	if err != nil {
		return preview, importlists.ErrUnavailable
	}
	defer rows.Close()
	preview.Rows = []importlists.Row{}
	for rows.Next() {
		var row importlists.Row
		var year sql.NullInt64
		var tmdbID sql.NullInt64
		if err = rows.Scan(&row.RowNumber, &row.SourceItemKey, &row.Title, &year, &tmdbID, &row.Status, &row.ErrorCode); err != nil {
			return preview, importlists.ErrUnavailable
		}
		if year.Valid {
			value := int(year.Int64)
			row.Year = &value
		}
		if tmdbID.Valid {
			value := tmdbID.Int64
			row.TMDBID = &value
		}
		preview.Rows = append(preview.Rows, row)
	}
	if err = rows.Err(); err != nil {
		return preview, importlists.ErrUnavailable
	}
	return preview, nil
}

func (s *Store) CommitImportListPreview(ctx context.Context, preview importlists.Preview, links []importlists.Link, created, duplicates int) (importlists.CommitResult, error) {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return importlists.CommitResult{}, importlists.ErrUnavailable
	}
	defer tx.Rollback()
	var state string
	var revision int
	var existingListID sql.NullInt64
	var existingLinked, existingCreated, existingDuplicates int
	err = tx.QueryRowContext(ctx, `SELECT state,revision,list_id,linked_count,created_count,duplicate_count FROM import_list_previews WHERE id=?`, preview.ID).Scan(&state, &revision, &existingListID, &existingLinked, &existingCreated, &existingDuplicates)
	if errors.Is(err, sql.ErrNoRows) {
		return importlists.CommitResult{}, importlists.ErrNotFound
	}
	if err != nil {
		return importlists.CommitResult{}, importlists.ErrUnavailable
	}
	if state == "committed" && existingListID.Valid {
		return importlists.CommitResult{PreviewID: preview.ID, ListID: existingListID.Int64, Revision: revision, Linked: existingLinked, Created: existingCreated, Duplicates: existingDuplicates, State: state}, nil
	}
	if revision != preview.Revision {
		return importlists.CommitResult{}, importlists.ErrChanged
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO import_lists(source_kind,list_key,name,revision,updated_at) VALUES(?,?,?,1,?) ON CONFLICT(source_kind,list_key) DO UPDATE SET name=excluded.name,revision=import_lists.revision+1,updated_at=excluded.updated_at`, preview.Source, preview.ListKey, preview.Name, now)
	if err != nil {
		return importlists.CommitResult{}, importlists.ErrUnavailable
	}
	var listID int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM import_lists WHERE source_kind=? AND list_key=?`, preview.Source, preview.ListKey).Scan(&listID); err != nil {
		return importlists.CommitResult{}, importlists.ErrUnavailable
	}
	for _, link := range links {
		_, err = tx.ExecContext(ctx, `INSERT INTO import_list_items(list_id,source_item_key,title,year,tmdb_id,movie_id,first_seen_at,last_seen_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(list_id,source_item_key) DO UPDATE SET title=excluded.title,year=excluded.year,tmdb_id=excluded.tmdb_id,movie_id=excluded.movie_id,last_seen_at=excluded.last_seen_at`, listID, link.SourceItemKey, link.Title, link.Year, link.TMDBID, link.MovieID, now, now)
		if err != nil {
			return importlists.CommitResult{}, importlists.ErrUnavailable
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE import_list_previews SET state='committed',revision=revision+1,committed_at=?,list_id=?,linked_count=?,created_count=?,duplicate_count=? WHERE id=? AND revision=? AND state='preview'`, now, listID, len(links), created, duplicates, preview.ID, preview.Revision)
	if err != nil {
		return importlists.CommitResult{}, importlists.ErrUnavailable
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return importlists.CommitResult{}, importlists.ErrChanged
	}
	if err = tx.Commit(); err != nil {
		return importlists.CommitResult{}, importlists.ErrUnavailable
	}
	return importlists.CommitResult{PreviewID: preview.ID, ListID: listID, Revision: preview.Revision + 1, Linked: len(links), Created: created, Duplicates: duplicates, State: "committed"}, nil
}

func (s *Store) ListImportLists(ctx context.Context) ([]importlists.ListItem, error) {
	rows, err := s.read.QueryContext(ctx, `SELECT l.id,l.source_kind,l.list_key,l.name,l.revision,COUNT(i.source_item_key),l.updated_at FROM import_lists l LEFT JOIN import_list_items i ON i.list_id=l.id GROUP BY l.id ORDER BY l.name COLLATE NOCASE,l.id LIMIT 100`)
	if err != nil {
		return nil, importlists.ErrUnavailable
	}
	defer rows.Close()
	items := []importlists.ListItem{}
	for rows.Next() {
		var item importlists.ListItem
		if err = rows.Scan(&item.ID, &item.Source, &item.ListKey, &item.Name, &item.Revision, &item.Items, &item.UpdatedAt); err != nil {
			return nil, importlists.ErrUnavailable
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, importlists.ErrUnavailable
	}
	return items, nil
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
