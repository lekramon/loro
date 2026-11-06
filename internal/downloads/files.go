package downloads

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/lekramon/piratarr/internal/downloadclient"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"
)

type remoteFile struct {
	Index    *int     `json:"index"`
	Name     *string  `json:"name"`
	Size     *int64   `json:"size"`
	Progress *float64 `json:"progress"`
	Priority *int     `json:"priority"`
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
func inside(root, name string) bool {
	rel, err := filepath.Rel(root, name)
	return err == nil && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func cleanRelative(value string) (string, bool) {
	if !utf8.ValidString(value) || len(value) == 0 || len(value) > 4096 || strings.ContainsRune(value, utf8.RuneError) {
		return "", false
	}
	value = strings.ReplaceAll(value, "\\", "/")
	parts := strings.Split(value, "/")
	if len(parts) > 64 {
		return "", false
	}
	for _, p := range parts {
		if p == "" || p == "." || p == ".." || strings.ContainsAny(p, `<>:"|?*`) || strings.TrimRight(p, ". ") != p {
			return "", false
		}
		for _, r := range p {
			if unicode.IsControl(r) {
				return "", false
			}
		}
		base := strings.ToUpper(strings.SplitN(p, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
			return "", false
		}
	}
	return strings.Join(parts, "/"), true
}
func validAbsolute(value string) bool {
	if len(value) == 0 || len(value) > 4096 || !utf8.ValidString(value) || !filepath.IsAbs(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func readFiles(ctx context.Context, s *downloadclient.Session, j Job, t transfer) (*Handoff, error) {
	if t.SavePath == nil || t.ContentPath == nil || !validAbsolute(j.Directory) || !validAbsolute(*t.SavePath) || !validAbsolute(*t.ContentPath) || !samePath(j.Directory, *t.SavePath) || !inside(j.Directory, *t.ContentPath) {
		return nil, Error("download-path-mismatch")
	}
	data, err := s.Exchange(ctx, "GET", "torrents/files?hash="+j.Hash, "", nil, 1<<20)
	if err != nil {
		return nil, err
	}
	var files []remoteFile
	if !utf8.Valid(data) || json.Unmarshal(data, &files) != nil || bytes.Equal(bytes.TrimSpace(data), []byte("null")) || len(files) > 10000 {
		return nil, invalid()
	}
	paths := map[string]bool{}
	indexes := map[int]bool{}
	candidates := []File{}
	var total int64
	incomplete := false
	for _, f := range files {
		if f.Index == nil || *f.Index < 0 || *f.Index > 9999 || indexes[*f.Index] || f.Name == nil || !validCount(f.Size) || f.Progress == nil || *f.Progress < 0 || *f.Progress > 1 || f.Priority == nil {
			return nil, invalid()
		}
		indexes[*f.Index] = true
		name, ok := cleanRelative(*f.Name)
		if !ok || paths[strings.ToLower(name)] {
			return nil, Error("download-file-path-unsafe")
		}
		paths[strings.ToLower(name)] = true
		full := filepath.Join(j.Directory, filepath.FromSlash(name))
		if !inside(j.Directory, full) || !samePath(*t.ContentPath, full) && !inside(*t.ContentPath, full) {
			return nil, Error("download-file-path-unsafe")
		}
		switch *f.Priority {
		case 0:
			continue
		case 1, 6, 7:
		default:
			return nil, invalid()
		}
		if *f.Size <= 0 || *f.Progress != 1 {
			incomplete = true
		}
		if *f.Size > maxInteger-total {
			return nil, invalid()
		}
		total += *f.Size
		// Match current inventory containers; an extension alone never verifies video/movie identity.
		switch strings.ToLower(filepath.Ext(name)) {
		case ".mkv", ".mp4", ".avi", ".mov", ".m4v", ".ts":
			if *f.Size > 0 && *f.Progress == 1 {
				candidates = append(candidates, File{Index: *f.Index, RelativePath: name, SizeBytes: *f.Size})
			}
		}
	}
	h := &Handoff{Version: 1, State: "needs-review"}
	reason := "download-file-ambiguous"
	h.ReasonCode = &reason
	if incomplete || total != *t.Size {
		reason = "download-files-incomplete"
		return h, nil
	}
	if len(candidates) == 1 {
		h.State = "awaiting-validation"
		h.ReasonCode = nil
		h.File = &candidates[0]
	}
	return h, nil
}
