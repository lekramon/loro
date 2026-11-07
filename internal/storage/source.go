package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/lekramon/piratarr/internal/sources"
)

func (s *Store) GetSource(ctx context.Context) (sources.Config, error) {
	return readSource(s.read.QueryRowContext(ctx, s.sourceQuery()))
}
func readSource(row scanner) (sources.Config, error) {
	result := sources.Config{Public: sources.Public{Kind: "torznab"}}
	var caps, at sql.NullString
	err := row.Scan(&result.Revision, &result.Name, &result.URL, &result.APIKey, &caps, &at, &result.Kind)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return sources.Config{}, sources.ErrUnavailable
	}
	result.Configured = true
	result.HasAPIKey = result.APIKey != ""
	if at.Valid {
		result.CheckedAt = &at.String
	}
	if caps.Valid {
		if json.Unmarshal([]byte(caps.String), &result.Capabilities) != nil {
			return sources.Config{}, sources.ErrUnavailable
		}
	}
	return result, nil
}
func (s *Store) SaveSource(ctx context.Context, revision int, name, endpoint, key string, kinds ...string) (sources.Config, error) {
	kind := "torznab"
	if len(kinds) > 0 {
		kind = kinds[0]
	}
	if kind != "torznab" && (kind != "1337x" || !s.nativeSourceSchema) {
		return sources.Config{}, sources.ErrValidation
	}
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return sources.Config{}, sources.ErrUnavailable
	}
	defer tx.Rollback()
	current, err := readSource(tx.QueryRowContext(ctx, s.sourceQuery()))
	if err != nil {
		return current, err
	}
	if current.Revision != revision {
		return sources.Config{}, sources.ErrChanged
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO source_configuration(id,revision,name,endpoint,api_key) VALUES(1,?,?,?,?) ON CONFLICT(id) DO UPDATE SET revision=excluded.revision,name=excluded.name,endpoint=excluded.endpoint,api_key=excluded.api_key,capabilities=NULL,checked_at=NULL`, revision+1, name, endpoint, key)
	if err != nil {
		return sources.Config{}, sources.ErrUnavailable
	}
	if s.nativeSourceSchema {
		if _, err = tx.ExecContext(ctx, "UPDATE source_configuration SET kind=? WHERE id=1", kind); err != nil {
			return sources.Config{}, sources.ErrUnavailable
		}
	}
	result, err := readSource(tx.QueryRowContext(ctx, s.sourceQuery()))
	if err != nil {
		return result, err
	}
	if err = tx.Commit(); err != nil {
		return sources.Config{}, sources.ErrUnavailable
	}
	return result, nil
}
func (s *Store) SaveSourceCapabilities(ctx context.Context, revision int, caps sources.Capabilities, at string) error {
	payload, err := json.Marshal(caps)
	if err != nil {
		return sources.ErrUnavailable
	}
	result, err := s.write.ExecContext(ctx, "UPDATE source_configuration SET capabilities=?,checked_at=? WHERE id=1 AND revision=?", string(payload), at, revision)
	if err != nil {
		return sources.ErrUnavailable
	}
	count, err := result.RowsAffected()
	if err != nil {
		return sources.ErrUnavailable
	}
	if count != 1 {
		return sources.ErrChanged
	}
	return nil
}

func (s *Store) sourceQuery() string {
	kind := "'torznab'"
	if s.nativeSourceSchema {
		kind = "kind"
	}
	return "SELECT revision,name,endpoint,api_key,capabilities,checked_at," + kind + " FROM source_configuration WHERE id=1"
}
