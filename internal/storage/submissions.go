package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lekramon/piratarr/internal/acquisitions"
	"github.com/lekramon/piratarr/internal/submissions"
)

const submissionSelect = "SELECT state,client_revision,attempts,error_code,updated_at FROM submission_jobs WHERE acquisition_id=?"

func readSubmission(row scanner) (*acquisitions.Submission, error) {
	var v acquisitions.Submission
	var code sql.NullString
	err := row.Scan(&v.State, &v.ClientRevision, &v.Attempts, &code, &v.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, acquisitions.ErrUnavailable
	}
	if code.Valid {
		v.ErrorCode = &code.String
	}
	if v.State == "succeeded" {
		v.State = "submitted"
	}
	if code.String == "submission-uncertain" && v.State == "needs-review" {
		v.State = "uncertain"
	}
	return &v, nil
}
func submissionRevisions(ctx context.Context, tx *sql.Tx, client, settings int) error {
	var c, p int
	if tx.QueryRowContext(ctx, "SELECT COALESCE((SELECT revision FROM download_client_configuration WHERE id=1),0),(SELECT revision FROM settings WHERE id=1)").Scan(&c, &p) != nil {
		return acquisitions.ErrUnavailable
	}
	if c != client {
		return acquisitions.Error("download-client-changed")
	}
	if p != settings {
		return acquisitions.Error("submission-configuration-changed")
	}
	return nil
}
func (s *Store) EnqueueSubmission(ctx context.Context, id string, revision, client, settings int, reconcile bool) (acquisitions.Acquisition, error) {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return acquisitions.Acquisition{}, acquisitions.ErrUnavailable
	}
	defer tx.Rollback()
	r, err := readAcquisition(tx.QueryRowContext(ctx, "SELECT "+acquisitionColumns+acquisitionJoin+" WHERE a.id=?", id))
	if err != nil {
		return r.Acquisition, err
	}
	if r.Revision != revision {
		return r.Acquisition, acquisitions.ErrChanged
	}
	previous, err := readSubmission(tx.QueryRowContext(ctx, submissionSelect, id))
	if err != nil {
		return r.Acquisition, err
	}
	now := time.Now().UTC()
	stamp := now.Format(time.RFC3339Nano)
	if previous != nil {
		r.Submission = previous
		if !reconcile || previous.State == "queued" || previous.State == "running" || previous.State == "submitted" {
			return r.Acquisition, nil
		}
		var pr int
		var due int64
		if tx.QueryRowContext(ctx, "SELECT settings_revision,due_at FROM submission_jobs WHERE acquisition_id=?", id).Scan(&pr, &due) != nil {
			return r.Acquisition, acquisitions.ErrUnavailable
		}
		if err = submissionRevisions(ctx, tx, previous.ClientRevision, pr); err != nil {
			return r.Acquisition, err
		}
		if due > now.UnixMilli() {
			return r.Acquisition, acquisitions.Error("submission-cooldown")
		}
		if _, err = tx.ExecContext(ctx, "UPDATE submission_jobs SET state='queued',error_code=NULL,lease_token=NULL,lease_expires=NULL,due_at=?,updated_at=? WHERE acquisition_id=?", now.UnixMilli(), stamp, id); err != nil {
			return r.Acquisition, acquisitions.ErrUnavailable
		}
	} else {
		if reconcile || r.State != "ready" || r.InfoHash == nil || r.Kind == nil {
			return r.Acquisition, acquisitions.ErrState
		}
		var cr, pr int
		var directory string
		if tx.QueryRowContext(ctx, "SELECT COALESCE((SELECT revision FROM download_client_configuration WHERE id=1),0),revision,download_directory FROM settings WHERE id=1").Scan(&cr, &pr, &directory) != nil {
			return r.Acquisition, acquisitions.ErrUnavailable
		}
		if pr != settings {
			return r.Acquisition, acquisitions.Error("submission-configuration-changed")
		}
		if cr == 0 {
			return r.Acquisition, acquisitions.Error("download-client-unconfigured")
		}
		if cr != client {
			return r.Acquisition, acquisitions.Error("download-client-changed")
		}
		var count int
		if tx.QueryRowContext(ctx, "SELECT count(*) FROM submission_jobs WHERE info_hash=?", *r.InfoHash).Scan(&count) != nil {
			return r.Acquisition, acquisitions.ErrUnavailable
		}
		if count > 0 {
			return r.Acquisition, acquisitions.Error("submission-hash-conflict")
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO submission_jobs(acquisition_id,info_hash,client_revision,settings_revision,download_directory,state,due_at,updated_at) VALUES(?,?,?,?,?,'queued',?,?)", id, *r.InfoHash, cr, pr, directory, now.UnixMilli(), stamp)
		if err != nil {
			return r.Acquisition, acquisitions.ErrUnavailable
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE acquisitions SET revision=revision+1,updated_at=? WHERE id=?", stamp, id); err != nil {
		return r.Acquisition, acquisitions.ErrUnavailable
	}
	r.Revision++
	r.UpdatedAt = stamp
	r.Submission, err = readSubmission(tx.QueryRowContext(ctx, submissionSelect, id))
	if err != nil {
		return r.Acquisition, err
	}
	if tx.Commit() != nil {
		return r.Acquisition, acquisitions.ErrUnavailable
	}
	return r.Acquisition, nil
}
func (s *Store) ClaimSubmission(ctx context.Context, token string, now time.Time) (*submissions.Job, error) {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return nil, acquisitions.ErrUnavailable
	}
	defer tx.Rollback()
	j := submissions.Job{Token: token}
	err = tx.QueryRowContext(ctx, "SELECT acquisition_id,info_hash,download_directory,client_revision,settings_revision,attempts,add_attempted FROM submission_jobs WHERE state='queued' AND due_at<=? ORDER BY due_at,acquisition_id LIMIT 1", now.UnixMilli()).Scan(&j.ID, &j.Hash, &j.Directory, &j.ClientRevision, &j.SettingsRevision, &j.Attempts, &j.Attempted)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, acquisitions.ErrUnavailable
	}
	_, err = tx.ExecContext(ctx, "UPDATE submission_jobs SET state='running',attempts=attempts+1,lease_token=?,lease_expires=?,updated_at=? WHERE acquisition_id=? AND state='queued'", token, now.Add(60*time.Second).UnixMilli(), now.Format(time.RFC3339Nano), j.ID)
	if err != nil || tx.Commit() != nil {
		return nil, acquisitions.ErrUnavailable
	}
	j.Attempts++
	return &j, nil
}
func fencedJob(ctx context.Context, tx *sql.Tx, j submissions.Job, now time.Time) error {
	var count int
	if tx.QueryRowContext(ctx, "SELECT count(*) FROM submission_jobs WHERE acquisition_id=? AND state='running' AND lease_token=? AND lease_expires>?", j.ID, j.Token, now.UnixMilli()).Scan(&count) != nil {
		return acquisitions.ErrUnavailable
	}
	if count != 1 {
		return acquisitions.ErrChanged
	}
	return nil
}
func (s *Store) RenewSubmission(ctx context.Context, j submissions.Job, now time.Time) error {
	return s.updateSubmissionLease(ctx, j, now, false)
}
func (s *Store) MarkSubmissionAttempt(ctx context.Context, j submissions.Job, now time.Time) error {
	return s.updateSubmissionLease(ctx, j, now, true)
}
func (s *Store) updateSubmissionLease(ctx context.Context, j submissions.Job, now time.Time, attempt bool) error {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return acquisitions.ErrUnavailable
	}
	defer tx.Rollback()
	if err = fencedJob(ctx, tx, j, now); err != nil {
		return err
	}
	if err = submissionRevisions(ctx, tx, j.ClientRevision, j.SettingsRevision); err != nil {
		return err
	}
	query := "UPDATE submission_jobs SET lease_expires=? WHERE acquisition_id=?"
	if attempt {
		query = "UPDATE submission_jobs SET lease_expires=?,add_attempted=1 WHERE acquisition_id=? AND add_attempted=0"
	}
	result, err := tx.ExecContext(ctx, query, now.Add(60*time.Second).UnixMilli(), j.ID)
	if err != nil {
		return acquisitions.ErrUnavailable
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return acquisitions.ErrChanged
	}
	if tx.Commit() != nil {
		return acquisitions.ErrUnavailable
	}
	return nil
}
func (s *Store) FinishSubmission(ctx context.Context, j submissions.Job, out submissions.Outcome, now time.Time) error {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return acquisitions.ErrUnavailable
	}
	defer tx.Rollback()
	if err = fencedJob(ctx, tx, j, now); err != nil {
		return err
	}
	if err = submissionRevisions(ctx, tx, j.ClientRevision, j.SettingsRevision); err != nil {
		var domain acquisitions.Error
		if !errors.As(err, &domain) || domain == acquisitions.ErrUnavailable {
			return acquisitions.ErrUnavailable
		}
		out.State = "needs-review"
		out.Code = string(domain)
	}
	if out.State != "succeeded" && out.State != "needs-review" {
		return acquisitions.ErrValidation
	}
	var code *string
	if out.Code != "" {
		code = &out.Code
	}
	stamp := now.Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, "UPDATE submission_jobs SET state=?,error_code=?,due_at=?,lease_token=NULL,lease_expires=NULL,updated_at=? WHERE acquisition_id=?", out.State, code, now.Add(time.Duration(out.Delay)*time.Second).UnixMilli(), stamp, j.ID); err != nil {
		return acquisitions.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, "UPDATE acquisitions SET revision=revision+1,updated_at=? WHERE id=?", stamp, j.ID); err != nil {
		return acquisitions.ErrUnavailable
	}
	if s.downloadSchema && out.State == "succeeded" {
		if err = createDownloadTask(ctx, tx, j.ID, now); err != nil {
			return acquisitions.ErrUnavailable
		}
	}
	if tx.Commit() != nil {
		return acquisitions.ErrUnavailable
	}
	return nil
}
