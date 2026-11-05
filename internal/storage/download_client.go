package storage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lekramon/piratarr/internal/downloadclient"
)

func (s *Store) GetDownloadClient(ctx context.Context) (downloadclient.Config, error) {
	return readDownloadClient(s.read.QueryRowContext(ctx, "SELECT revision,endpoint,username,password,app_version,api_version,checked_at FROM download_client_configuration WHERE id=1"))
}
func readDownloadClient(row scanner) (downloadclient.Config, error) {
	var result downloadclient.Config
	var app, api, at sql.NullString
	err := row.Scan(&result.Revision, &result.URL, &result.Username, &result.Password, &app, &api, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return downloadclient.Config{}, downloadclient.ErrUnavailable
	}
	result.Configured = true
	result.HasPassword = result.Password != ""
	if at.Valid {
		result.CheckedAt = &at.String
	}
	if app.Valid {
		result.AppVersion = &app.String
	}
	if api.Valid {
		result.APIVersion = &api.String
	}
	return result, nil
}
func (s *Store) SaveDownloadClient(ctx context.Context, revision int, endpoint, username, password string) (downloadclient.Config, error) {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return downloadclient.Config{}, downloadclient.ErrUnavailable
	}
	defer tx.Rollback()
	current, err := readDownloadClient(tx.QueryRowContext(ctx, "SELECT revision,endpoint,username,password,app_version,api_version,checked_at FROM download_client_configuration WHERE id=1"))
	if err != nil {
		return current, err
	}
	if current.Revision != revision {
		return downloadclient.Config{}, downloadclient.ErrChanged
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO download_client_configuration(id,revision,endpoint,username,password) VALUES(1,?,?,?,?) ON CONFLICT(id) DO UPDATE SET revision=excluded.revision,endpoint=excluded.endpoint,username=excluded.username,password=excluded.password,app_version=NULL,api_version=NULL,checked_at=NULL`, revision+1, endpoint, username, password)
	if err != nil {
		return downloadclient.Config{}, downloadclient.ErrUnavailable
	}
	result, err := readDownloadClient(tx.QueryRowContext(ctx, "SELECT revision,endpoint,username,password,app_version,api_version,checked_at FROM download_client_configuration WHERE id=1"))
	if err != nil {
		return result, err
	}
	if err = tx.Commit(); err != nil {
		return downloadclient.Config{}, downloadclient.ErrUnavailable
	}
	return result, nil
}
func (s *Store) SaveDownloadClientTest(ctx context.Context, revision int, app, api, at string) error {
	result, err := s.write.ExecContext(ctx, "UPDATE download_client_configuration SET app_version=?,api_version=?,checked_at=? WHERE id=1 AND revision=?", app, api, at, revision)
	if err != nil {
		return downloadclient.ErrUnavailable
	}
	count, err := result.RowsAffected()
	if err != nil {
		return downloadclient.ErrUnavailable
	}
	if count != 1 {
		return downloadclient.ErrChanged
	}
	return nil
}
