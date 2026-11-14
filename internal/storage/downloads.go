package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/lekramon/piratarr/internal/acquisitions"
	"github.com/lekramon/piratarr/internal/downloads"
	"time"
)

const taskColumns = "a.snapshot,j.info_hash,j.client_revision,d.acquisition_id,d.revision,d.state,d.snapshot,d.error_code,d.checked_at,d.observed_at,d.completed_at,d.handoff,d.poll_state,d.due_at,d.updated_at"
const taskJoin = " FROM download_tasks d JOIN submission_jobs j ON j.acquisition_id=d.acquisition_id JOIN acquisitions a ON a.id=d.acquisition_id"

func readDownload(row scanner) (downloads.Task, error) {
	var t downloads.Task
	var acquisition, metrics string
	var code, checked, observed, completed, handoff sql.NullString
	var due sql.NullInt64
	err := row.Scan(&acquisition, &t.InfoHash, &t.ClientRevision, &t.AcquisitionID, &t.Revision, &t.State, &metrics, &code, &checked, &observed, &completed, &handoff, &t.PollState, &due, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return t, downloads.ErrNotFound
	}
	if err != nil {
		return t, downloads.ErrUnavailable
	}
	var a acquisitions.Acquisition
	if json.Unmarshal([]byte(acquisition), &a) != nil || json.Unmarshal([]byte(metrics), &t.Metrics) != nil {
		return t, downloads.ErrUnavailable
	}
	t.MovieID = a.MovieID
	t.EpisodeID = a.EpisodeID
	t.MovieTitle = a.MovieTitle
	t.ReleaseTitle = a.Candidate.Title
	if code.Valid {
		t.ErrorCode = &code.String
	}
	if checked.Valid {
		t.LastCheckedAt = &checked.String
	}
	if observed.Valid {
		t.LastObservedAt = &observed.String
	}
	if completed.Valid {
		t.CompletedAt = &completed.String
	}
	if handoff.Valid {
		if json.Unmarshal([]byte(handoff.String), &t.Handoff) != nil {
			return t, downloads.ErrUnavailable
		}
	}
	if t.PollState == "queued" && due.Valid {
		value := time.UnixMilli(due.Int64).UTC().Format(time.RFC3339Nano)
		t.NextCheckAt = &value
	}
	return t, nil
}
func (s *Store) ListDownloads(ctx context.Context, limit, offset int, movie int64) (downloads.List, error) {
	result := downloads.List{Items: []downloads.Task{}, Limit: limit, Offset: offset}
	filter := " WHERE (?=0 OR a.movie_id=?)"
	if s.read.QueryRowContext(ctx, "SELECT count(*)"+taskJoin+filter, movie, movie).Scan(&result.Total) != nil {
		return result, downloads.ErrUnavailable
	}
	rows, err := s.read.QueryContext(ctx, "SELECT "+taskColumns+taskJoin+filter+" ORDER BY d.updated_at DESC,d.acquisition_id DESC LIMIT ? OFFSET ?", movie, movie, limit, offset)
	if err != nil {
		return result, downloads.ErrUnavailable
	}
	defer rows.Close()
	for rows.Next() {
		t, e := readDownload(rows)
		if e != nil {
			return result, e
		}
		result.Items = append(result.Items, t)
	}
	if rows.Err() != nil {
		return result, downloads.ErrUnavailable
	}
	return result, nil
}
func (s *Store) GetDownload(ctx context.Context, id string) (downloads.Detail, error) {
	var result downloads.Detail
	result.Events = []downloads.Event{}
	tx, err := s.read.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return result, downloads.ErrUnavailable
	}
	defer tx.Rollback()
	result.Download, err = readDownload(tx.QueryRowContext(ctx, "SELECT "+taskColumns+taskJoin+" WHERE d.acquisition_id=?", id))
	if err != nil {
		return result, err
	}
	if tx.QueryRowContext(ctx, "SELECT count(*) FROM download_events WHERE acquisition_id=?", id).Scan(&result.EventTotal) != nil {
		return result, downloads.ErrUnavailable
	}
	rows, err := tx.QueryContext(ctx, "SELECT state,code,progress,handoff_state,created_at FROM download_events WHERE acquisition_id=? ORDER BY id DESC LIMIT 50", id)
	if err != nil {
		return result, downloads.ErrUnavailable
	}
	for rows.Next() {
		var e downloads.Event
		var code, handoff sql.NullString
		var progress sql.NullFloat64
		if rows.Scan(&e.State, &code, &progress, &handoff, &e.CreatedAt) != nil {
			rows.Close()
			return result, downloads.ErrUnavailable
		}
		if code.Valid {
			e.Code = &code.String
		}
		if handoff.Valid {
			e.HandoffState = &handoff.String
		}
		if progress.Valid {
			e.Progress = &progress.Float64
		}
		result.Events = append(result.Events, e)
	}
	rowErr := rows.Err()
	rows.Close()
	if rowErr != nil || tx.Commit() != nil {
		return result, downloads.ErrUnavailable
	}
	return result, nil
}
func createDownloadTask(ctx context.Context, tx *sql.Tx, id string, now time.Time) error {
	stamp := now.UTC().Format(time.RFC3339Nano)
	r, err := tx.ExecContext(ctx, "INSERT INTO download_tasks(acquisition_id,due_at,updated_at) VALUES(?,?,?) ON CONFLICT(acquisition_id) DO NOTHING", id, now.UnixMilli(), stamp)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 1 {
		_, err = tx.ExecContext(ctx, "INSERT INTO download_events(acquisition_id,state,created_at) VALUES(?,'pending',?)", id, stamp)
	}
	return err
}
func downloadRevisions(ctx context.Context, tx *sql.Tx, client, settings int) error {
	err := submissionRevisions(ctx, tx, client, settings)
	if err == nil {
		return nil
	}
	var domain acquisitions.Error
	if errors.As(err, &domain) {
		return downloads.Error(domain)
	}
	return downloads.ErrUnavailable
}
func (s *Store) QueueDownloadCheck(ctx context.Context, id string, revision int) (downloads.Task, error) {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return downloads.Task{}, downloads.ErrUnavailable
	}
	defer tx.Rollback()
	t, err := readDownload(tx.QueryRowContext(ctx, "SELECT "+taskColumns+taskJoin+" WHERE d.acquisition_id=?", id))
	if err != nil {
		return t, err
	}
	if t.Revision != revision {
		return t, downloads.ErrChanged
	}
	now := time.Now().UTC()
	if t.PollState == "running" {
		return t, nil
	}
	if t.PollState == "queued" {
		if t.NextCheckAt != nil && instant(*t.NextCheckAt) > now.UnixMilli() {
			return t, downloads.ErrCooldown
		}
		return t, nil
	}
	var settings int
	if tx.QueryRowContext(ctx, "SELECT settings_revision FROM submission_jobs WHERE acquisition_id=? AND state='succeeded'", id).Scan(&settings) != nil {
		return t, downloads.ErrUnavailable
	}
	if err = downloadRevisions(ctx, tx, t.ClientRevision, settings); err != nil {
		return t, err
	}
	stamp := now.Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, "UPDATE download_tasks SET state='pending',revision=revision+1,poll_state='queued',due_at=?,handoff=NULL,completed_at=NULL,error_code=NULL,updated_at=? WHERE acquisition_id=?", now.UnixMilli(), stamp, id); err != nil {
		return t, downloads.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO download_events(acquisition_id,state,progress,created_at) VALUES(?,'pending',?,?)", id, t.Progress, stamp); err != nil {
		return t, downloads.ErrUnavailable
	}
	t, err = readDownload(tx.QueryRowContext(ctx, "SELECT "+taskColumns+taskJoin+" WHERE d.acquisition_id=?", id))
	if err != nil || tx.Commit() != nil {
		return t, downloads.ErrUnavailable
	}
	return t, nil
}
func (s *Store) ClaimDownloadPoll(ctx context.Context, token string, now time.Time) (*downloads.Job, error) {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return nil, downloads.ErrUnavailable
	}
	defer tx.Rollback()
	j := downloads.Job{Token: token}
	err = tx.QueryRowContext(ctx, "SELECT d.acquisition_id,j.info_hash,j.download_directory,j.client_revision,j.settings_revision FROM download_tasks d JOIN submission_jobs j ON j.acquisition_id=d.acquisition_id WHERE d.poll_state='queued' AND d.due_at<=? AND j.state='succeeded' ORDER BY d.due_at,d.acquisition_id LIMIT 1", now.UnixMilli()).Scan(&j.ID, &j.Hash, &j.Directory, &j.ClientRevision, &j.SettingsRevision)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, downloads.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, "UPDATE download_tasks SET poll_state='running',attempts=attempts+1,lease_token=?,lease_expires=?,updated_at=? WHERE acquisition_id=?", token, now.Add(60*time.Second).UnixMilli(), now.UTC().Format(time.RFC3339Nano), j.ID); err != nil || tx.Commit() != nil {
		return nil, downloads.ErrUnavailable
	}
	return &j, nil
}
func pollFence(ctx context.Context, tx *sql.Tx, j downloads.Job, now time.Time) error {
	var count int
	if tx.QueryRowContext(ctx, "SELECT count(*) FROM download_tasks d JOIN submission_jobs j ON j.acquisition_id=d.acquisition_id WHERE d.acquisition_id=? AND d.poll_state='running' AND d.lease_token=? AND d.lease_expires>? AND j.state='succeeded'", j.ID, j.Token, now.UnixMilli()).Scan(&count) != nil {
		return downloads.ErrUnavailable
	}
	if count != 1 {
		return downloads.ErrChanged
	}
	return nil
}
func (s *Store) RenewDownloadPoll(ctx context.Context, j downloads.Job, now time.Time) error {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return downloads.ErrUnavailable
	}
	defer tx.Rollback()
	if err = pollFence(ctx, tx, j, now); err != nil {
		return err
	}
	if err = downloadRevisions(ctx, tx, j.ClientRevision, j.SettingsRevision); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE download_tasks SET lease_expires=? WHERE acquisition_id=?", now.Add(60*time.Second).UnixMilli(), j.ID); err != nil || tx.Commit() != nil {
		return downloads.ErrUnavailable
	}
	return nil
}
func (s *Store) FinishDownloadPoll(ctx context.Context, j downloads.Job, out downloads.Result, now time.Time) error {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return downloads.ErrUnavailable
	}
	defer tx.Rollback()
	if err = pollFence(ctx, tx, j, now); err != nil {
		return err
	}
	if err = downloadRevisions(ctx, tx, j.ClientRevision, j.SettingsRevision); err != nil {
		if err == downloads.ErrUnavailable {
			return err
		}
		out = downloads.Result{State: "needs-review", Code: err.Error()}
	}
	old, err := readDownload(tx.QueryRowContext(ctx, "SELECT "+taskColumns+taskJoin+" WHERE d.acquisition_id=?", j.ID))
	if err != nil {
		return err
	}
	switch out.State {
	case "pending", "queued", "downloading", "paused", "unavailable", "completed", "failed", "needs-review":
	default:
		return downloads.ErrValidation
	}
	metrics := old.Metrics
	observed := old.LastObservedAt
	stamp := now.UTC().Format(time.RFC3339Nano)
	if out.Metrics != nil {
		metrics = *out.Metrics
		observed = &stamp
	}
	body, err := json.Marshal(metrics)
	if err != nil {
		return downloads.ErrUnavailable
	}
	var code *string
	if out.Code != "" {
		code = &out.Code
	}
	var handoff *string
	if out.Handoff != nil {
		data, e := json.Marshal(out.Handoff)
		if e != nil {
			return downloads.ErrUnavailable
		}
		str := string(data)
		handoff = &str
	}
	poll := "idle"
	var due *int64
	switch out.State {
	case "pending", "queued", "downloading", "paused", "unavailable":
		poll = "queued"
		delay := max(15, out.RetryAfter)
		delay = min(delay, 3600)
		value := now.Add(time.Duration(delay) * time.Second).UnixMilli()
		due = &value
	}
	if _, err = tx.ExecContext(ctx, "UPDATE download_tasks SET state=?,revision=revision+1,snapshot=?,error_code=?,checked_at=?,observed_at=?,completed_at=?,handoff=?,poll_state=?,due_at=?,lease_token=NULL,lease_expires=NULL,updated_at=? WHERE acquisition_id=?", out.State, string(body), code, stamp, observed, out.CompletedAt, handoff, poll, due, stamp, j.ID); err != nil {
		return downloads.ErrUnavailable
	}
	oldHandoff, _ := json.Marshal(old.Handoff)
	newHandoff, _ := json.Marshal(out.Handoff)
	if old.State != out.State || stringValue(old.ErrorCode) != out.Code || string(oldHandoff) != string(newHandoff) {
		var state *string
		eventCode := code
		if out.Handoff != nil {
			state = &out.Handoff.State
			if eventCode == nil {
				eventCode = out.Handoff.ReasonCode
			}
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO download_events(acquisition_id,state,code,progress,handoff_state,created_at) VALUES(?,?,?,?,?,?)", j.ID, out.State, eventCode, metrics.Progress, state, stamp); err != nil {
			return downloads.ErrUnavailable
		}
	}
	if tx.Commit() != nil {
		return downloads.ErrUnavailable
	}
	return nil
}
func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
