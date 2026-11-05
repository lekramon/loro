package library

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Review struct {
	Revision        int    `json:"revision"`
	Action          string `json:"action"`
	MovieID         *int64 `json:"movieId,omitempty"`
	ConfirmIdentity bool   `json:"confirmIdentity,omitempty"`
}
type ReviewRepository interface {
	Repository
	InventoryFile(context.Context, string, int64) (Entry, error)
	CommitReview(context.Context, string, int, int64, Review) (Entry, error)
}

var yearPattern = regexp.MustCompile(`(?:^|[ ._(\[-])((?:18|19|20)[0-9]{2})(?:[ ._)\]-]|$)`)

func Suggest(entry *Entry) {
	name := strings.TrimSuffix(filepath.Base(entry.RelativePath), filepath.Ext(entry.RelativePath))
	switch strings.ToLower(name) {
	case "movie", "video", "main", "feature":
		if parent := filepath.Dir(entry.RelativePath); parent != "." {
			name = filepath.Base(parent)
		}
	}
	if match := yearPattern.FindStringSubmatchIndex(name); match != nil {
		year, _ := strconv.Atoi(name[match[2]:match[3]])
		if year >= 1870 && year <= 2100 {
			entry.SuggestedYear = &year
			name = name[:match[0]]
		}
	}
	entry.SuggestedTitle = strings.Trim(strings.NewReplacer(".", " ", "_", " ").Replace(name), " -_([")
}
func (s *Service) Review(ctx context.Context, id int64, input Review) (Entry, error) {
	if input.Revision < 1 || (input.Action == "associate" && (input.MovieID == nil || *input.MovieID < 1 || !input.ConfirmIdentity)) {
		return Entry{}, ErrConflict
	}
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	default:
		return Entry{}, ErrBusy
	}
	repo, ok := s.repo.(ReviewRepository)
	if !ok {
		return Entry{}, ErrUnavailable
	}
	config, err := repo.GetSettings(ctx)
	if err != nil {
		return Entry{}, err
	}
	path := filepath.Clean(config.Directories.MovieDirectory)
	entry, err := repo.InventoryFile(ctx, path, id)
	if err != nil {
		return entry, err
	}
	if entry.Revision != input.Revision {
		return Entry{}, ErrConflict
	}
	if input.Action == "associate" {
		if entry.Status != "present" || entry.ReviewState == "ignored" {
			return Entry{}, ErrConflict
		}
		if entry.Size <= 0 {
			return Entry{}, ErrInvalidVideo
		}
		canonical, err := filepath.EvalSymlinks(path)
		if err != nil || PathKey(canonical) != PathKey(path) || overlaps(path, s.data) || overlaps(s.data, path) {
			return Entry{}, ErrConflict
		}
		before, err := os.Lstat(path)
		if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
			return Entry{}, ErrConflict
		}
		root, err := os.OpenRoot(path)
		if err != nil {
			return Entry{}, ErrConflict
		}
		defer root.Close()
		rootInfo, err := root.Stat(".")
		if err != nil || !os.SameFile(before, rootInfo) {
			return Entry{}, ErrConflict
		}
		relative := filepath.FromSlash(entry.RelativePath)
		if !filepath.IsLocal(relative) {
			return Entry{}, ErrConflict
		}
		current := ""
		for _, part := range strings.Split(relative, string(filepath.Separator)) {
			current = filepath.Join(current, part)
			info, err := root.Lstat(current)
			if err != nil || info.Mode()&os.ModeSymlink != 0 {
				return Entry{}, ErrConflict
			}
		}
		file, err := root.Open(relative)
		if err != nil {
			return Entry{}, ErrConflict
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() != entry.Size || info.ModTime().UTC().Format(time.RFC3339Nano) != entry.ModifiedAt {
			return Entry{}, ErrConflict
		}
		if err = s.prober(ctx, file); err != nil {
			return Entry{}, err
		}
		after, err := file.Stat()
		if err != nil || !os.SameFile(info, after) || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
			return Entry{}, ErrConflict
		}
		current = ""
		for _, part := range strings.Split(relative, string(filepath.Separator)) {
			current = filepath.Join(current, part)
			component, err := root.Lstat(current)
			if err != nil || component.Mode()&os.ModeSymlink != 0 {
				return Entry{}, ErrConflict
			}
		}
		seen, err := root.Lstat(relative)
		if err != nil || seen.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, seen) {
			return Entry{}, ErrConflict
		}
		now, err := os.Lstat(path)
		canonical, resolveErr := filepath.EvalSymlinks(path)
		if err != nil || resolveErr != nil || PathKey(canonical) != PathKey(path) || !os.SameFile(before, now) {
			return Entry{}, ErrConflict
		}
	}
	return repo.CommitReview(ctx, path, config.Revision, id, input)
}
