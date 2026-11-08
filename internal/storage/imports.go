package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lekramon/piratarr/internal/imports"
)

func (s *Store) Preview(ctx context.Context, id string, revision int) (imports.Preview, error) {
	var p imports.Preview
	var handoff, metadata sql.NullString
	var state, downloadRoot, movieRoot string
	var movieID int64
	var size int64
	err := s.read.QueryRowContext(ctx, `SELECT d.revision,d.state,d.handoff,j.download_directory,m.title,m.metadata,m.id,s.movie_directory,COALESCE(json_extract(d.handoff,'$.file.relativePath'),''),COALESCE(json_extract(d.handoff,'$.file.sizeBytes'),0) FROM download_tasks d JOIN submission_jobs j ON j.acquisition_id=d.acquisition_id JOIN acquisitions a ON a.id=d.acquisition_id JOIN movies m ON m.id=a.movie_id JOIN settings s ON s.id=1 WHERE d.acquisition_id=?`, id).Scan(&p.Revision, &state, &handoff, &downloadRoot, &p.MovieTitle, &metadata, &movieID, &movieRoot, &p.RelativePath, &size)
	if errors.Is(err, sql.ErrNoRows) { return p, imports.ErrNotFound }
	if err != nil { return p, imports.ErrUnavailable }
	if p.Revision != revision || state != "completed" { return p, imports.ErrChanged }
	if !handoff.Valid || handoff.String == "" || p.RelativePath == "" || size <= 0 { return p, imports.ErrReview }
	p.DownloadID, p.MovieID, p.SizeBytes, p.DestinationRoot = id, movieID, size, movieRoot
	p.SourcePath = filepath.Join(downloadRoot, filepath.FromSlash(p.RelativePath))
	if !insideImportRoot(downloadRoot, p.SourcePath) { return p, imports.ErrReview }
	if info, statErr := os.Stat(p.SourcePath); statErr != nil || !info.Mode().IsRegular() { return p, imports.ErrReview }
	var movieMetadata struct { Year *int `json:"year"` }
	if metadata.Valid && json.Unmarshal([]byte(metadata.String), &movieMetadata) == nil { p.MovieYear = movieMetadata.Year }
	return p, nil
}

func insideImportRoot(root, name string) bool {
	r, err := filepath.Abs(root); if err != nil { return false }
	n, err := filepath.Abs(name); if err != nil { return false }
	rel, err := filepath.Rel(r, n)
	return err == nil && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (s *Store) List(ctx context.Context, limit, offset int) (imports.List, error) {
	out := imports.List{Items: []imports.Record{}, Limit: limit, Offset: offset}
	if err := s.read.QueryRowContext(ctx, "SELECT count(*) FROM import_jobs").Scan(&out.Total); err != nil { return out, imports.ErrUnavailable }
	rows, err := s.read.QueryContext(ctx, "SELECT id,download_id,movie_id,relative_path,state,phase,revision,created_at,updated_at FROM import_jobs ORDER BY updated_at DESC,id DESC LIMIT ? OFFSET ?", limit, offset)
	if err != nil { return out, imports.ErrUnavailable }
	defer rows.Close()
	for rows.Next() { var record imports.Record; if err := rows.Scan(&record.ID, &record.DownloadID, &record.MovieID, &record.RelativePath, &record.State, &record.Phase, &record.Revision, &record.CreatedAt, &record.UpdatedAt); err != nil { return out, imports.ErrUnavailable }; out.Items = append(out.Items, record) }
	return out, rows.Err()
}

func (s *Store) Plan(ctx context.Context, in imports.Input, p imports.Preview, destination string) (imports.Record, error) {
	id, now := randomImportID(), time.Now().UTC().Format(time.RFC3339Nano)
	relative := filepath.ToSlash(strings.TrimPrefix(destination, p.DestinationRoot+string(filepath.Separator)))
	_, err := s.write.ExecContext(ctx, "INSERT INTO import_jobs(id,download_id,movie_id,source_path,destination_path,relative_path,state,phase,revision,created_at,updated_at) VALUES(?,?,?,?,?,?,?,'planned',1,?,?)", id, in.DownloadID, in.MovieID, p.SourcePath, destination, relative, "staging", now, now)
	if err != nil { return imports.Record{}, imports.ErrUnavailable }
	return imports.Record{ID: id, DownloadID: in.DownloadID, MovieID: in.MovieID, RelativePath: relative, State: "planned", Phase: "planned", Revision: 1, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *Store) Commit(ctx context.Context, id string, revision int, destination, relative string, size int64, modified string) (imports.Record, error) {
	tx, err := s.write.BeginTx(ctx, nil); if err != nil { return imports.Record{}, imports.ErrUnavailable }; defer tx.Rollback()
	var record imports.Record
	err = tx.QueryRowContext(ctx, "SELECT id,download_id,movie_id,relative_path,revision FROM import_jobs WHERE id=? AND revision=?", id, revision).Scan(&record.ID, &record.DownloadID, &record.MovieID, &record.RelativePath, &record.Revision)
	if errors.Is(err, sql.ErrNoRows) { return record, imports.ErrChanged }; if err != nil { return record, imports.ErrUnavailable }
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, "UPDATE import_jobs SET state='committed',phase='committed',revision=revision+1,updated_at=? WHERE id=? AND revision=?", now, id, revision); err != nil { return record, imports.ErrUnavailable }
	var root string; if err = tx.QueryRowContext(ctx, "SELECT movie_directory FROM settings WHERE id=1").Scan(&root); err != nil { return record, imports.ErrUnavailable }
	key := filepath.ToSlash(relative)
	if _, err = tx.ExecContext(ctx, "INSERT INTO library_files(root_key,path_key,relative_path,size,modified_at,present,first_seen_at,last_seen_at,revision,review_state,movie_id,verified_at) VALUES(?,?,?, ?,?,1,?,?,1,'associated',?,?) ON CONFLICT(root_key,path_key) DO UPDATE SET size=excluded.size,modified_at=excluded.modified_at,present=1,revision=revision+1,review_state='associated',movie_id=excluded.movie_id,verified_at=excluded.verified_at", root, strings.ToLower(key), key, size, modified, now, now, record.MovieID, now); err != nil { return record, imports.ErrUnavailable }
	if err = tx.Commit(); err != nil { return record, imports.ErrUnavailable }
	record.State, record.Phase, record.Revision, record.UpdatedAt = "committed", "committed", revision+1, now
	return record, nil
}

func (s *Store) MarkNeedsReview(ctx context.Context, id string, revision int, code string) error {
	result, err := s.write.ExecContext(ctx, "UPDATE import_jobs SET state='needs-review',phase='needs-review',error_code=?,revision=revision+1,updated_at=? WHERE id=? AND revision=?", code, time.Now().UTC().Format(time.RFC3339Nano), id, revision)
	if err != nil { return imports.ErrUnavailable }; count, _ := result.RowsAffected(); if count != 1 { return imports.ErrChanged }; return nil
}

func randomImportID() string { return strings.ToLower(strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")) + "0000000000000000" }
