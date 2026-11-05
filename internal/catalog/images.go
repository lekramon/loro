package catalog

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var filenamePattern = regexp.MustCompile(`^[A-Za-z0-9]+\.(jpg|png)$`)
var cacheNamePattern = regexp.MustCompile(`^[a-f0-9]{64}\.img$`)

func imageType(data []byte) (string, error) {
	if len(data) > 8<<20 {
		return "", &ProviderError{Code: "metadata-invalid-response"}
	}
	configuration, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || configuration.Width < 1 || configuration.Height < 1 || configuration.Width > 4096 || configuration.Height > 4096 {
		return "", &ProviderError{Code: "metadata-invalid-response"}
	}
	switch format {
	case "jpeg":
		return "image/jpeg", nil
	case "png":
		return "image/png", nil
	}
	return "", &ProviderError{Code: "metadata-invalid-response"}
}
func (s *Service) Image(ctx context.Context, size, filename string) ([]byte, string, error) {
	if (size != "w342" && size != "w780") || !filenamePattern.MatchString(filename) {
		return nil, "", ErrNotFound
	}
	known, err := s.Repo.KnownArtwork(ctx, ImagePrefix+size+"/"+filename)
	if err != nil {
		return nil, "", err
	}
	if !known {
		return nil, "", ErrNotFound
	}
	select {
	case s.imageGate <- struct{}{}:
		defer func() { <-s.imageGate }()
	case <-ctx.Done():
		return nil, "", &ProviderError{Code: "metadata-timeout"}
	}
	directory, err := s.imageDirectory()
	if err != nil {
		return nil, "", ErrUnavailable
	}
	digest := sha256.Sum256([]byte(size + "/" + filename))
	path := filepath.Join(directory, fmt.Sprintf("%x.img", digest))
	if info, e := os.Lstat(path); e == nil && info.Mode().IsRegular() && info.Size() <= 8<<20 {
		if data, e := os.ReadFile(path); e == nil {
			if mime, e := imageType(data); e == nil {
				_ = os.Chtimes(path, time.Now(), time.Now())
				return data, mime, nil
			}
		}
	}
	data, err := s.Provider.Image(ctx, size, filename)
	if err != nil {
		return nil, "", err
	}
	mime, err := imageType(data)
	if err != nil {
		return nil, "", err
	}
	file, err := os.CreateTemp(directory, ".incoming-")
	if err != nil {
		return nil, "", ErrUnavailable
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return nil, "", ErrUnavailable
	}
	if info, e := os.Lstat(path); e == nil {
		if !info.Mode().IsRegular() {
			return nil, "", ErrUnavailable
		}
		if err = os.Remove(path); err != nil {
			return nil, "", ErrUnavailable
		}
	}
	if err = os.Rename(temporary, path); err != nil {
		return nil, "", ErrUnavailable
	}
	limit := s.ImageCacheBytes
	if limit <= 0 {
		limit = 128 << 20
	}
	if err = pruneImages(directory, path, limit); err != nil {
		return nil, "", ErrUnavailable
	}
	return data, mime, nil
}
func (s *Service) imageDirectory() (string, error) {
	root, err := filepath.EvalSymlinks(s.DataDirectory)
	if err != nil {
		return "", err
	}
	current := root
	for _, part := range []string{"cache", "tmdb"} {
		current = filepath.Join(current, part)
		if err = os.Mkdir(current, 0700); err != nil && !os.IsExist(err) {
			return "", err
		}
		info, e := os.Lstat(current)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", ErrUnavailable
		}
	}
	resolved, err := filepath.EvalSymlinks(current)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", ErrUnavailable
	}
	return resolved, nil
}
func pruneImages(directory, keep string, limit int64) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	type cached struct {
		path string
		size int64
		at   time.Time
	}
	files := []cached{}
	var total int64
	for _, entry := range entries {
		if !cacheNamePattern.MatchString(entry.Name()) || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			continue
		}
		total += info.Size()
		files = append(files, cached{filepath.Join(directory, entry.Name()), info.Size(), info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].at.Before(files[j].at) })
	for _, file := range files {
		if total <= limit {
			break
		}
		if file.path == keep {
			continue
		}
		if err = os.Remove(file.path); err != nil {
			return err
		}
		total -= file.size
	}
	return nil
}
