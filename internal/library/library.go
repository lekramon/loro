// Package library observes existing media without changing user files.
package library

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lekramon/piratarr/internal/settings"
)

var (
	ErrBusy        = errors.New("scan in progress")
	ErrLimit       = errors.New("scan limit exceeded")
	ErrUnavailable = errors.New("scan unavailable")
	ErrChanged     = errors.New("settings changed")
)

type Observation struct {
	RelativePath string
	Size         int64
	ModifiedAt   string
}
type Entry struct {
	ID             int64   `json:"id"`
	RelativePath   string  `json:"relativePath"`
	Size           int64   `json:"size"`
	ModifiedAt     string  `json:"modifiedAt"`
	Status         string  `json:"status"`
	FirstSeenAt    string  `json:"firstSeenAt"`
	LastSeenAt     string  `json:"lastSeenAt"`
	Revision       int     `json:"revision"`
	ReviewState    string  `json:"reviewState"`
	MovieID        *int64  `json:"movieId"`
	MovieTitle     string  `json:"movieTitle"`
	VerifiedAt     *string `json:"verifiedAt"`
	SuggestedTitle string  `json:"suggestedTitle"`
	SuggestedYear  *int    `json:"suggestedYear"`
}
type Summary struct {
	ScannedAt      string `json:"scannedAt"`
	FilesPresent   int    `json:"filesPresent"`
	SkippedEntries int    `json:"skippedEntries"`
}
type Snapshot struct {
	Root   string   `json:"root"`
	Items  []Entry  `json:"items"`
	Total  int      `json:"total"`
	Limit  int      `json:"limit"`
	Offset int      `json:"offset"`
	Scan   *Summary `json:"scan"`
}
type Repository interface {
	GetSettings(context.Context) (settings.Settings, error)
	PublishInventory(context.Context, string, int, []Observation, int) error
	ListInventory(context.Context, string, int, int, string) (Snapshot, error)
}
type Service struct {
	repo                           Repository
	prober                         Prober
	data                           string
	gate                           chan struct{}
	maxEntries, maxFiles, maxDepth int
}

func New(repo Repository, data string, probers ...Prober) *Service {
	prober := NewProber("ffprobe")
	if len(probers) > 0 {
		prober = probers[0]
	}
	return &Service{prober: prober, repo: repo, data: data, gate: make(chan struct{}, 1), maxEntries: 100000, maxFiles: 10000, maxDepth: 64}
}
func PathKey(value string) string {
	value = filepath.Clean(value)
	if runtime.GOOS == "windows" {
		value = strings.ToLower(value)
	}
	return value
}
func overlaps(a, b string) bool {
	a = PathKey(a)
	b = PathKey(b)
	rel, err := filepath.Rel(a, b)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func (s *Service) List(ctx context.Context, limit, offset int, status string) (Snapshot, error) {
	config, err := s.repo.GetSettings(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	return s.repo.ListInventory(ctx, config.Directories.MovieDirectory, limit, offset, status)
}
func (s *Service) Scan(ctx context.Context) (Snapshot, error) {
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	default:
		return Snapshot{}, ErrBusy
	}
	config, err := s.repo.GetSettings(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	path := filepath.Clean(config.Directories.MovieDirectory)
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil || PathKey(canonical) != PathKey(path) || overlaps(path, s.data) || overlaps(s.data, path) {
		return Snapshot{}, ErrUnavailable
	}
	before, err := os.Lstat(path)
	if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return Snapshot{}, ErrUnavailable
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return Snapshot{}, ErrUnavailable
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(before, opened) {
		return Snapshot{}, ErrUnavailable
	}
	entries, skipped := 0, 0
	observed := []Observation{}
	var walk func(string, int) error
	walk = func(relative string, depth int) error {
		if ctx.Err() != nil {
			return ErrUnavailable
		}
		if depth > s.maxDepth {
			return ErrLimit
		}
		expected, err := root.Lstat(relative)
		if err != nil || !expected.IsDir() || expected.Mode()&os.ModeSymlink != 0 {
			return ErrUnavailable
		}
		dir, err := root.Open(relative)
		if err != nil {
			return ErrUnavailable
		}
		defer dir.Close()
		current, err := dir.Stat()
		if err != nil || !os.SameFile(expected, current) {
			return ErrUnavailable
		}
		for {
			batch, readErr := dir.ReadDir(128)
			if readErr != nil && readErr != io.EOF {
				return ErrUnavailable
			}
			for _, entry := range batch {
				if ctx.Err() != nil {
					return ErrUnavailable
				}
				entries++
				if entries > s.maxEntries {
					return ErrLimit
				}
				rel := filepath.Join(relative, entry.Name())
				if !utf8.ValidString(rel) {
					skipped++
					continue
				}
				info, err := root.Lstat(rel)
				if err != nil {
					return ErrUnavailable
				}
				if info.Mode()&os.ModeSymlink != 0 {
					skipped++
					continue
				}
				if info.IsDir() {
					if err = walk(rel, depth+1); err != nil {
						return err
					}
					continue
				}
				if !info.Mode().IsRegular() {
					skipped++
					continue
				}
				switch strings.ToLower(filepath.Ext(rel)) {
				case ".mkv", ".mp4", ".avi", ".mov", ".m4v", ".ts":
				default:
					skipped++
					continue
				}
				if len(observed) >= s.maxFiles {
					return ErrLimit
				}
				observed = append(observed, Observation{RelativePath: filepath.ToSlash(rel), Size: info.Size(), ModifiedAt: info.ModTime().UTC().Format(time.RFC3339Nano)})
			}
			if readErr == io.EOF {
				final, err := root.Lstat(relative)
				if err != nil || final.Mode()&os.ModeSymlink != 0 || !os.SameFile(expected, final) {
					return ErrUnavailable
				}
				return nil
			}
		}
	}
	if err = walk(".", 0); err != nil {
		return Snapshot{}, err
	}
	after, err := os.Lstat(path)
	canonical, resolveErr := filepath.EvalSymlinks(path)
	if err != nil || resolveErr != nil || PathKey(canonical) != PathKey(path) || !os.SameFile(before, after) || ctx.Err() != nil {
		return Snapshot{}, ErrUnavailable
	}
	if err = s.repo.PublishInventory(ctx, path, config.Revision, observed, skipped); err != nil {
		return Snapshot{}, err
	}
	return s.repo.ListInventory(ctx, path, 50, 0, "all")
}
