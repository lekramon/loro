package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/lekramon/piratarr/internal/sources"
)

func (s *Store) GetSource(ctx context.Context) (sources.Config, error) {
	return readSource(s.read.QueryRowContext(ctx, s.sourceQuery()), s.multiSourceSchema)
}

func readSource(row scanner, multiple bool) (sources.Config, error) {
	result := sources.Config{Public: sources.Public{Kind: "torznab"}}
	var caps, at sql.NullString
	var err error
	if multiple {
		var active int
		err = row.Scan(&result.ID, &active, &result.Revision, &result.Name, &result.URL, &result.APIKey, &caps, &at, &result.Kind)
		result.Active = active == 1
	} else {
		err = row.Scan(&result.Revision, &result.Name, &result.URL, &result.APIKey, &caps, &at, &result.Kind)
		result.ID = 1
		result.Active = true
	}
	if errors.Is(err, sql.ErrNoRows) {
		return sources.Config{Public: sources.Public{Kind: "torznab"}}, nil
	}
	if err != nil {
		return sources.Config{}, sources.ErrUnavailable
	}
	result.Configured = true
	result.HasAPIKey = result.APIKey != ""
	if at.Valid {
		result.CheckedAt = &at.String
	}
	if caps.Valid && json.Unmarshal([]byte(caps.String), &result.Capabilities) != nil {
		return sources.Config{}, sources.ErrUnavailable
	}
	return result, nil
}

func (s *Store) ListSources(ctx context.Context) ([]sources.Config, error) {
	if !s.multiSourceSchema {
		config, err := s.GetSource(ctx)
		if err != nil || !config.Configured {
			return []sources.Config{}, err
		}
		return []sources.Config{config}, nil
	}
	rows, err := s.read.QueryContext(ctx, "SELECT id,active,revision,name,endpoint,api_key,capabilities,checked_at,kind FROM source_configuration ORDER BY lower(name),id")
	if err != nil {
		return nil, sources.ErrUnavailable
	}
	defer rows.Close()
	result := []sources.Config{}
	for rows.Next() {
		config, err := readSource(rows, true)
		if err != nil {
			return nil, err
		}
		result = append(result, config)
	}
	if rows.Err() != nil {
		return nil, sources.ErrUnavailable
	}
	return result, nil
}

func nextSourceRevision(ctx context.Context, tx *sql.Tx) (int, error) {
	var revision int
	if err := tx.QueryRowContext(ctx, "SELECT coalesce(max(revision),0)+1 FROM source_configuration").Scan(&revision); err != nil {
		return 0, sources.ErrUnavailable
	}
	return revision, nil
}

func (s *Store) CreateSource(ctx context.Context, name, endpoint, key, kind string) (sources.Config, error) {
	if !s.multiSourceSchema {
		return sources.Config{}, sources.ErrUnavailable
	}
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return sources.Config{}, sources.ErrUnavailable
	}
	defer tx.Rollback()
	var duplicate int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM source_configuration WHERE endpoint=?", endpoint).Scan(&duplicate); err != nil {
		return sources.Config{}, sources.ErrUnavailable
	}
	if duplicate != 0 {
		return sources.Config{}, sources.ErrExists
	}
	revision, err := nextSourceRevision(ctx, tx)
	if err != nil {
		return sources.Config{}, err
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO source_configuration(active,revision,name,endpoint,api_key,kind) VALUES(0,?,?,?,?,?)", revision, name, endpoint, key, kind)
	if err != nil {
		return sources.Config{}, sources.ErrUnavailable
	}
	id, err := result.LastInsertId()
	if err != nil {
		return sources.Config{}, sources.ErrUnavailable
	}
	created, err := readSource(tx.QueryRowContext(ctx, "SELECT id,active,revision,name,endpoint,api_key,capabilities,checked_at,kind FROM source_configuration WHERE id=?", id), true)
	if err != nil {
		return sources.Config{}, err
	}
	if tx.Commit() != nil {
		return sources.Config{}, sources.ErrUnavailable
	}
	return created, nil
}

func (s *Store) ActivateSource(ctx context.Context, id int64, revision int) (sources.Config, error) {
	if !s.multiSourceSchema {
		return sources.Config{}, sources.ErrUnavailable
	}
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return sources.Config{}, sources.ErrUnavailable
	}
	defer tx.Rollback()
	current, err := readSource(tx.QueryRowContext(ctx, "SELECT id,active,revision,name,endpoint,api_key,capabilities,checked_at,kind FROM source_configuration WHERE id=?", id), true)
	if err != nil {
		return sources.Config{}, err
	}
	if !current.Configured {
		return sources.Config{}, sources.ErrNotFound
	}
	if current.Revision != revision {
		return sources.Config{}, sources.ErrChanged
	}
	if current.Active {
		return current, nil
	}
	next, err := nextSourceRevision(ctx, tx)
	if err != nil {
		return sources.Config{}, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE source_configuration SET active=0 WHERE active=1"); err != nil {
		return sources.Config{}, sources.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, "UPDATE source_configuration SET active=1,revision=? WHERE id=?", next, id); err != nil {
		return sources.Config{}, sources.ErrUnavailable
	}
	current, err = readSource(tx.QueryRowContext(ctx, "SELECT id,active,revision,name,endpoint,api_key,capabilities,checked_at,kind FROM source_configuration WHERE id=?", id), true)
	if err != nil {
		return sources.Config{}, err
	}
	if tx.Commit() != nil {
		return sources.Config{}, sources.ErrUnavailable
	}
	return current, nil
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
	current, err := readSource(tx.QueryRowContext(ctx, s.sourceQuery()), s.multiSourceSchema)
	if err != nil {
		return current, err
	}
	if current.Revision != revision {
		return sources.Config{}, sources.ErrChanged
	}
	next := revision + 1
	if s.multiSourceSchema {
		next, err = nextSourceRevision(ctx, tx)
		if err != nil {
			return sources.Config{}, err
		}
		var duplicate int
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM source_configuration WHERE endpoint=? AND id<>?", endpoint, current.ID).Scan(&duplicate); err != nil {
			return sources.Config{}, sources.ErrUnavailable
		}
		if duplicate != 0 {
			return sources.Config{}, sources.ErrExists
		}
		if current.Configured {
			_, err = tx.ExecContext(ctx, "UPDATE source_configuration SET revision=?,name=?,endpoint=?,api_key=?,capabilities=NULL,checked_at=NULL,kind=? WHERE id=?", next, name, endpoint, key, kind, current.ID)
		} else {
			_, err = tx.ExecContext(ctx, "INSERT INTO source_configuration(active,revision,name,endpoint,api_key,kind) VALUES(1,?,?,?,?,?)", next, name, endpoint, key, kind)
		}
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO source_configuration(id,revision,name,endpoint,api_key) VALUES(1,?,?,?,?) ON CONFLICT(id) DO UPDATE SET revision=excluded.revision,name=excluded.name,endpoint=excluded.endpoint,api_key=excluded.api_key,capabilities=NULL,checked_at=NULL`, next, name, endpoint, key)
		if err == nil && s.nativeSourceSchema {
			_, err = tx.ExecContext(ctx, "UPDATE source_configuration SET kind=? WHERE id=1", kind)
		}
	}
	if err != nil {
		return sources.Config{}, sources.ErrUnavailable
	}
	result, err := readSource(tx.QueryRowContext(ctx, s.sourceQuery()), s.multiSourceSchema)
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
	where := "id=1 AND revision=?"
	if s.multiSourceSchema {
		where = "active=1 AND revision=?"
	}
	result, err := s.write.ExecContext(ctx, "UPDATE source_configuration SET capabilities=?,checked_at=? WHERE "+where, string(payload), at, revision)
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
	if s.multiSourceSchema {
		return "SELECT id,active,revision,name,endpoint,api_key,capabilities,checked_at,kind FROM source_configuration WHERE active=1"
	}
	kind := "'torznab'"
	if s.nativeSourceSchema {
		kind = "kind"
	}
	return "SELECT revision,name,endpoint,api_key,capabilities,checked_at," + kind + " FROM source_configuration WHERE id=1"
}
