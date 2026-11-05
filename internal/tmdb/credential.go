package tmdb

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/lekramon/piratarr/internal/bootstrap"
)

var ErrInvalidToken = errors.New("invalid metadata credential")

// SaveToken persists a write-only service override. Existing credentials survive failure.
func (c *Client) SaveToken(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 8192 {
		return ErrInvalidToken
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-') {
			return ErrInvalidToken
		}
	}
	if c.config.LocalTokenFile == "" {
		return errors.New("credential storage unavailable")
	}
	directory := filepath.Dir(c.config.LocalTokenFile)
	// Never follow a redirected credential directory.
	if info, err := os.Lstat(directory); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return errors.New("credential storage unavailable")
	}
	if _, err := bootstrap.PrepareDataDir(directory); err != nil {
		return errors.New("credential storage unavailable")
	}
	file, err := os.CreateTemp(directory, ".tmdb-*")
	if err != nil {
		return errors.New("credential storage unavailable")
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(0600); err == nil {
		_, err = file.WriteString(value)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return errors.New("credential storage unavailable")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := os.Rename(file.Name(), c.config.LocalTokenFile); err != nil {
		return errors.New("credential storage unavailable")
	}
	return nil
}
