package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/lekramon/piratarr/internal/acquisitions"
)

func revisionsMatch(ctx context.Context, tx *sql.Tx, source, profile int) error {
	var sr, pr int
	if err := tx.QueryRowContext(ctx, "SELECT (SELECT revision FROM source_configuration WHERE id=1),(SELECT revision FROM settings WHERE id=1)").Scan(&sr, &pr); err != nil {
		return acquisitions.ErrUnavailable
	}
	if sr != source || pr != profile {
		return acquisitions.ErrObservationChanged
	}
	return nil
}
func instant(value string) int64 { t, _ := time.Parse(time.RFC3339Nano, value); return t.UnixMilli() }
func (s *Store) SaveObservations(ctx context.Context, values []acquisitions.Observation, source, profile int, now string) error {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return acquisitions.ErrUnavailable
	}
	defer tx.Rollback()
	if err = revisionsMatch(ctx, tx, source, profile); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM release_observations WHERE expires_at<=? AND NOT EXISTS(SELECT 1 FROM acquisitions WHERE observation_id=release_observations.id)", instant(now)); err != nil {
		return acquisitions.ErrUnavailable
	}
	for _, value := range values {
		payload, err := json.Marshal(value)
		if err != nil {
			return acquisitions.ErrUnavailable
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO release_observations(id,movie_id,source_revision,profile_revision,snapshot,source_url,reference,hash_claim,reference_issue,expires_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, value.ID, value.MovieID, value.SourceRevision, value.ProfileRevision, string(payload), value.SourceURL, value.Reference, value.HashClaim, value.ReferenceIssue, instant(value.ExpiresAt))
		if err != nil {
			return acquisitions.ErrUnavailable
		}
	}
	if tx.Commit() != nil {
		return acquisitions.ErrUnavailable
	}
	return nil
}

const acquisitionColumnsBase = `a.snapshot,a.state,a.revision,a.info_hash,a.payload_kind,a.review_code,a.created_at,a.updated_at,o.source_url,o.reference,o.hash_claim,o.reference_issue`
const acquisitionColumns = acquisitionColumnsBase + ",a.payload"
const acquisitionJoin = ` FROM acquisitions a JOIN release_observations o ON o.id=a.observation_id`

func readAcquisition(row scanner) (acquisitions.Record, error) {
	var record acquisitions.Record
	var snapshot, state, created, updated string
	var revision int
	var hash, kind, review sql.NullString
	err := row.Scan(&snapshot, &state, &revision, &hash, &kind, &review, &created, &updated, &record.SourceURL, &record.Reference, &record.HashClaim, &record.ReferenceIssue, &record.Payload)
	if errors.Is(err, sql.ErrNoRows) {
		return record, acquisitions.ErrNotFound
	}
	if err != nil || json.Unmarshal([]byte(snapshot), &record.Acquisition) != nil {
		return acquisitions.Record{}, acquisitions.ErrUnavailable
	}
	record.State = state
	record.Revision = revision
	record.CreatedAt = created
	record.UpdatedAt = updated
	if hash.Valid {
		record.InfoHash = &hash.String
	}
	if kind.Valid {
		record.Kind = &kind.String
		if kind.String == "magnet" && strings.HasPrefix(record.Reference, "louro-1337x:") && strings.HasPrefix(string(record.Payload), "magnet:?") {
			record.Reference = string(record.Payload)
		}
	}
	if review.Valid {
		record.ReviewCode = &review.String
	}
	return record, nil
}
func (s *Store) GetAcquisition(ctx context.Context, id string) (acquisitions.Record, error) {
	record, err := readAcquisition(s.read.QueryRowContext(ctx, "SELECT "+acquisitionColumns+acquisitionJoin+" WHERE a.id=?", id))
	if err == nil && s.submissionSchema {
		record.Submission, err = readSubmission(s.read.QueryRowContext(ctx, submissionSelect, id))
	}
	return record, err
}
func (s *Store) ListAcquisitions(ctx context.Context, limit, offset int, movie int64) (acquisitions.List, error) {
	result := acquisitions.List{Items: []acquisitions.Acquisition{}, Limit: limit, Offset: offset}
	filter := " WHERE (?=0 OR a.movie_id=?)"
	if s.read.QueryRowContext(ctx, "SELECT count(*) FROM acquisitions a"+filter, movie, movie).Scan(&result.Total) != nil {
		return result, acquisitions.ErrUnavailable
	}
	rows, err := s.read.QueryContext(ctx, "SELECT "+(acquisitionColumnsBase+",NULL")+acquisitionJoin+filter+" ORDER BY a.created_at DESC,a.id DESC LIMIT ? OFFSET ?", movie, movie, limit, offset)
	if err != nil {
		return result, acquisitions.ErrUnavailable
	}
	defer rows.Close()
	for rows.Next() {
		record, err := readAcquisition(rows)
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, record.Acquisition)
	}
	if rows.Err() != nil {
		return result, acquisitions.ErrUnavailable
	}
	rows.Close()
	if s.submissionSchema {
		for i := range result.Items {
			result.Items[i].Submission, err = readSubmission(s.read.QueryRowContext(ctx, submissionSelect, result.Items[i].ID))
			if err != nil {
				return result, err
			}
		}
	}
	return result, nil
}
func (s *Store) SelectAcquisition(ctx context.Context, input acquisitions.Selection, id, now string) (acquisitions.Acquisition, bool, error) {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return acquisitions.Acquisition{}, false, acquisitions.ErrUnavailable
	}
	defer tx.Rollback()
	var previousID, observationID string
	var previousOverride bool
	err = tx.QueryRowContext(ctx, "SELECT acquisition_id,observation_id,confirm_override FROM acquisition_requests WHERE request_key=?", input.IdempotencyKey).Scan(&previousID, &observationID, &previousOverride)
	if err == nil {
		if observationID != input.ObservationID || previousOverride != input.ConfirmOverride {
			return acquisitions.Acquisition{}, false, acquisitions.ErrIdempotency
		}
		record, err := readAcquisition(tx.QueryRowContext(ctx, "SELECT "+acquisitionColumns+acquisitionJoin+" WHERE a.id=?", previousID))
		return record.Acquisition, false, err
	}
	if err != sql.ErrNoRows {
		return acquisitions.Acquisition{}, false, acquisitions.ErrUnavailable
	}
	var observation acquisitions.Observation
	var snapshot string
	var expires int64
	err = tx.QueryRowContext(ctx, "SELECT snapshot,expires_at FROM release_observations WHERE id=?", input.ObservationID).Scan(&snapshot, &expires)
	if err == sql.ErrNoRows {
		return acquisitions.Acquisition{}, false, acquisitions.ErrObservationNotFound
	}
	if err != nil || json.Unmarshal([]byte(snapshot), &observation) != nil {
		return acquisitions.Acquisition{}, false, acquisitions.ErrUnavailable
	}
	if expires <= instant(now) {
		return acquisitions.Acquisition{}, false, acquisitions.ErrExpired
	}
	if err = revisionsMatch(ctx, tx, observation.SourceRevision, observation.ProfileRevision); err != nil {
		return acquisitions.Acquisition{}, false, err
	}
	if observation.Candidate.Status != "match" && !input.ConfirmOverride {
		return acquisitions.Acquisition{}, false, acquisitions.ErrConfirmation
	}
	created := false
	err = tx.QueryRowContext(ctx, "SELECT id FROM acquisitions WHERE movie_id=? AND candidate_id=? AND state!='cancelled'", observation.MovieID, observation.Candidate.ID).Scan(&previousID)
	if err == sql.ErrNoRows {
		previousID = id
		created = true
		value := acquisitions.Acquisition{ID: id, MovieID: observation.MovieID, MovieTitle: observation.MovieTitle, OriginalLanguage: observation.OriginalLanguage, SourceName: observation.SourceName, SourceRevision: observation.SourceRevision, ProfileRevision: observation.ProfileRevision, Candidate: observation.Candidate, Profile: observation.Profile, OverrideConfirmed: observation.Candidate.Status != "match", State: "selected", Revision: 1, CreatedAt: now, UpdatedAt: now}
		data, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			return acquisitions.Acquisition{}, false, acquisitions.ErrUnavailable
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO acquisitions(id,observation_id,movie_id,candidate_id,snapshot,state,revision,created_at,updated_at) VALUES(?,?,?,?,?,'selected',1,?,?)`, id, input.ObservationID, observation.MovieID, observation.Candidate.ID, string(data), now, now)
	}
	if err != nil {
		return acquisitions.Acquisition{}, false, acquisitions.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO acquisition_requests(request_key,observation_id,confirm_override,acquisition_id) VALUES(?,?,?,?)", input.IdempotencyKey, input.ObservationID, input.ConfirmOverride, previousID); err != nil {
		return acquisitions.Acquisition{}, false, acquisitions.ErrUnavailable
	}
	record, err := readAcquisition(tx.QueryRowContext(ctx, "SELECT "+acquisitionColumns+acquisitionJoin+" WHERE a.id=?", previousID))
	if err != nil {
		return acquisitions.Acquisition{}, false, err
	}
	if tx.Commit() != nil {
		return acquisitions.Acquisition{}, false, acquisitions.ErrUnavailable
	}
	return record.Acquisition, created, nil
}
func (s *Store) BeginPreparation(ctx context.Context, id string, revision int, now string) (acquisitions.Record, error) {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return acquisitions.Record{}, acquisitions.ErrUnavailable
	}
	defer tx.Rollback()
	record, err := readAcquisition(tx.QueryRowContext(ctx, "SELECT "+acquisitionColumns+acquisitionJoin+" WHERE a.id=?", id))
	if err != nil {
		return record, err
	}
	if s.submissionSchema {
		var count int
		if tx.QueryRowContext(ctx, "SELECT count(*) FROM submission_jobs WHERE acquisition_id=?", id).Scan(&count) != nil {
			return record, acquisitions.ErrUnavailable
		}
		if count > 0 {
			return record, acquisitions.Error("submission-exists")
		}
	}
	if record.Revision != revision {
		return record, acquisitions.ErrChanged
	}
	if record.State == "ready" {
		return record, nil
	}
	if record.State != "selected" && record.State != "needs-review" {
		return record, acquisitions.ErrState
	}
	var source int
	if tx.QueryRowContext(ctx, "SELECT revision FROM source_configuration WHERE id=1").Scan(&source) != nil {
		return record, acquisitions.ErrUnavailable
	}
	state := "preparing"
	var review *string
	if source != record.SourceRevision {
		state = "needs-review"
		code := "source-changed"
		review = &code
	}
	_, err = tx.ExecContext(ctx, "UPDATE acquisitions SET state=?,revision=revision+1,review_code=?,info_hash=NULL,payload_kind=NULL,payload=NULL,updated_at=? WHERE id=? AND revision=?", state, review, now, id, revision)
	if err != nil {
		return record, acquisitions.ErrUnavailable
	}
	record.State = state
	record.Revision++
	record.ReviewCode = review
	record.UpdatedAt = now
	if tx.Commit() != nil {
		return acquisitions.Record{}, acquisitions.ErrUnavailable
	}
	return record, nil
}
func (s *Store) FinishPreparation(ctx context.Context, id string, revision int, value acquisitions.Preparation, now string) (acquisitions.Acquisition, error) {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return acquisitions.Acquisition{}, acquisitions.ErrUnavailable
	}
	defer tx.Rollback()
	record, err := readAcquisition(tx.QueryRowContext(ctx, "SELECT "+acquisitionColumns+acquisitionJoin+" WHERE a.id=?", id))
	if err != nil {
		return record.Acquisition, err
	}
	if record.State != "preparing" || record.Revision != revision {
		return record.Acquisition, acquisitions.ErrChanged
	}
	var source int
	if tx.QueryRowContext(ctx, "SELECT revision FROM source_configuration WHERE id=1").Scan(&source) != nil {
		return record.Acquisition, acquisitions.ErrUnavailable
	}
	if source != record.SourceRevision {
		value = acquisitions.Preparation{ReviewCode: "source-changed"}
	}
	state := "ready"
	var hash, kind, review *string
	if value.ReviewCode != "" {
		state = "needs-review"
		review = &value.ReviewCode
		value.Payload = nil
	} else {
		hash = &value.Hash
		kind = &value.Kind
	}
	_, err = tx.ExecContext(ctx, "UPDATE acquisitions SET state=?,revision=revision+1,info_hash=?,payload_kind=?,review_code=?,payload=?,updated_at=? WHERE id=? AND revision=? AND state='preparing'", state, hash, kind, review, value.Payload, now, id, revision)
	if err != nil {
		return record.Acquisition, acquisitions.ErrUnavailable
	}
	record.State = state
	record.Revision++
	record.InfoHash = hash
	record.Kind = kind
	record.ReviewCode = review
	record.UpdatedAt = now
	if tx.Commit() != nil {
		return acquisitions.Acquisition{}, acquisitions.ErrUnavailable
	}
	return record.Acquisition, nil
}
func (s *Store) CancelAcquisition(ctx context.Context, id string, revision int, now string) (acquisitions.Acquisition, error) {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return acquisitions.Acquisition{}, acquisitions.ErrUnavailable
	}
	defer tx.Rollback()
	record, err := readAcquisition(tx.QueryRowContext(ctx, "SELECT "+acquisitionColumns+acquisitionJoin+" WHERE a.id=?", id))
	if err != nil {
		return record.Acquisition, err
	}
	if s.submissionSchema {
		var count int
		if tx.QueryRowContext(ctx, "SELECT count(*) FROM submission_jobs WHERE acquisition_id=?", id).Scan(&count) != nil {
			return record.Acquisition, acquisitions.ErrUnavailable
		}
		if count > 0 {
			return record.Acquisition, acquisitions.Error("submission-exists")
		}
	}
	if record.Revision != revision {
		return record.Acquisition, acquisitions.ErrChanged
	}
	if record.State == "cancelled" {
		return record.Acquisition, nil
	}
	if _, err = tx.ExecContext(ctx, "UPDATE acquisitions SET state='cancelled',revision=revision+1,updated_at=? WHERE id=? AND revision=?", now, id, revision); err != nil {
		return record.Acquisition, acquisitions.ErrUnavailable
	}
	record.State = "cancelled"
	record.Revision++
	record.UpdatedAt = now
	if tx.Commit() != nil {
		return acquisitions.Acquisition{}, acquisitions.ErrUnavailable
	}
	return record.Acquisition, nil
}
