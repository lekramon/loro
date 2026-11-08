// Package operations owns offline, local maintenance workflows.
package operations

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

const (
	ManifestPath = "manifest.json"
	SchemaVersion = 14
	MaxArchiveBytes = 512 << 20
)

var ErrInvalidArchive = errors.New("invalid operations archive")

type Manifest struct {
	FormatVersion int `json:"formatVersion"`
	SchemaVersion int `json:"schemaVersion"`
	CreatedAt string `json:"createdAt"`
	Files []File `json:"files"`
}

type File struct {
	Path string `json:"path"`
	Size int64 `json:"size"`
	SHA256 string `json:"sha256"`
}

func Backup(dataDir, output string) error {
	lock := flock.New(filepath.Join(dataDir, "piratarr.lock"))
	locked, err := lock.TryLock()
	if err != nil { return fmt.Errorf("acquire data lock: %w", err) }
	if !locked { return errors.New("service must be stopped before backup") }
	defer lock.Unlock()

	files, err := collect(dataDir)
	if err != nil { return err }
	manifest := Manifest{FormatVersion: 1, SchemaVersion: SchemaVersion, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Files: files}
	return writeArchive(dataDir, output, manifest)
}

func Check(input string) (Manifest, error) {
	archive, err := zip.OpenReader(input)
	if err != nil { return Manifest{}, ErrInvalidArchive }
	defer archive.Close()
	manifest, err := inspect(archive.File)
	if err != nil { return Manifest{}, err }
	if err = verifyEntries(archive.File, manifest); err != nil { return Manifest{}, err }
	return manifest, nil
}

func Restore(input, destination string) error {
	if _, err := os.Stat(destination); err == nil {
		entries, readErr := os.ReadDir(destination)
		if readErr != nil || len(entries) != 0 { return errors.New("restore destination must be empty") }
	} else if !os.IsNotExist(err) { return err }
	archive, err := zip.OpenReader(input)
	if err != nil { return ErrInvalidArchive }
	defer archive.Close()
	manifest, err := inspect(archive.File)
	if err != nil { return err }
	if manifest.SchemaVersion > SchemaVersion { return errors.New("archive schema is newer than this application") }
	if err = verifyEntries(archive.File, manifest); err != nil { return err }
	parent := filepath.Dir(filepath.Clean(destination))
	if err = os.MkdirAll(parent, 0700); err != nil { return err }
	stage := filepath.Join(parent, filepath.Base(destination)+".louro-restore-staging")
	if _, err = os.Stat(stage); err == nil { return errors.New("restore staging path already exists") }
	if err = os.Mkdir(stage, 0700); err != nil { return err }
	defer os.RemoveAll(stage)
	if err = extractAndVerify(archive.File, stage, manifest); err != nil { return err }
	if _, statErr := os.Stat(destination); statErr == nil {
		// The destination was verified empty above. Move it aside so publication
		// remains a rename and can be rolled back if the second rename fails.
		emptyBackup := destination + ".louro-empty"
		if err = os.Rename(destination, emptyBackup); err != nil { return fmt.Errorf("stage restore destination: %w", err) }
		if err = os.Rename(stage, destination); err != nil {
			_ = os.Rename(emptyBackup, destination)
			return fmt.Errorf("publish restore: %w", err)
		}
		_ = os.Remove(emptyBackup)
		return nil
	}
	if err = os.Rename(stage, destination); err != nil { return fmt.Errorf("publish restore: %w", err) }
	return nil
}

func collect(root string) ([]File, error) {
	var out []File
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil { return walkErr }
		if path == root { return nil }
		rel, err := filepath.Rel(root, path); if err != nil { return err }
		if entry.IsDir() {
			if rel == "media" || rel == "downloads" || rel == "cache" { return filepath.SkipDir }
			return nil
		}
		info, err := entry.Info(); if err != nil { return err }
		if info.Mode()&os.ModeSymlink != 0 { return fmt.Errorf("symlink is not allowed: %s", rel) }
		if !info.Mode().IsRegular() || rel == "piratarr.lock" { return nil }
		digest, size, err := digestFile(path); if err != nil { return err }
		out = append(out, File{Path: filepath.ToSlash(rel), Size: size, SHA256: digest})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, err
}

func writeArchive(root, output string, manifest Manifest) error {
	file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil { return err }
	defer file.Close()
	archive := zip.NewWriter(file)
	manifestBytes, err := json.Marshal(manifest); if err != nil { return err }
	entry, err := archive.Create(ManifestPath); if err != nil { return err }
	if _, err = entry.Write(manifestBytes); err != nil { return err }
	for _, item := range manifest.Files {
		entry, err = archive.Create(item.Path); if err != nil { return err }
		input, err := os.Open(filepath.Join(root, filepath.FromSlash(item.Path))); if err != nil { return err }
		_, copyErr := io.Copy(entry, input); input.Close(); if copyErr != nil { return copyErr }
	}
	if err = archive.Close(); err != nil { return err }
	return file.Close()
}

func inspect(entries []*zip.File) (Manifest, error) {
	if len(entries) == 0 { return Manifest{}, ErrInvalidArchive }
	var manifest Manifest
	seen := map[string]bool{}
	for _, entry := range entries {
		name := filepath.ToSlash(entry.Name)
		if name == "" || filepath.IsAbs(name) || strings.HasPrefix(name, "../") || name == ".." || strings.Contains(name, "/../") || seen[name] { return Manifest{}, ErrInvalidArchive }
		seen[name] = true
		if name != ManifestPath && (entry.FileInfo().Mode()&os.ModeSymlink) != 0 { return Manifest{}, ErrInvalidArchive }
		if name == ManifestPath {
			reader, err := entry.Open(); if err != nil { return Manifest{}, ErrInvalidArchive }
			decodeErr := json.NewDecoder(io.LimitReader(reader, 4<<20)).Decode(&manifest); reader.Close(); if decodeErr != nil { return Manifest{}, ErrInvalidArchive }
		}
	}
	if manifest.FormatVersion != 1 || manifest.SchemaVersion < 1 { return Manifest{}, ErrInvalidArchive }
	if len(manifest.Files) == 0 { return Manifest{}, ErrInvalidArchive }
	seenManifest := map[string]bool{}
	for _, item := range manifest.Files {
		if item.Path == ManifestPath || item.Path == "" || filepath.IsAbs(item.Path) || strings.HasPrefix(item.Path, "../") || strings.Contains(item.Path, "/../") || seenManifest[item.Path] || item.Size < 0 || item.Size > MaxArchiveBytes || len(item.SHA256) != sha256.Size*2 { return Manifest{}, ErrInvalidArchive }
		seenManifest[item.Path] = true
	}
	return manifest, nil
}

func verifyEntries(entries []*zip.File, manifest Manifest) error {
	wanted := map[string]File{}
	for _, item := range manifest.Files { wanted[item.Path] = item }
	var total int64
	for _, entry := range entries {
		name := filepath.ToSlash(entry.Name)
		if name == ManifestPath { continue }
		item, ok := wanted[name]
		if !ok || entry.FileInfo().Mode().IsDir() || entry.UncompressedSize64 > uint64(MaxArchiveBytes) { return ErrInvalidArchive }
		total += int64(entry.UncompressedSize64)
		if total > MaxArchiveBytes { return ErrInvalidArchive }
		reader, err := entry.Open(); if err != nil { return ErrInvalidArchive }
		digest := sha256.New()
		size, copyErr := io.Copy(digest, io.LimitReader(reader, MaxArchiveBytes+1)); reader.Close()
		if copyErr != nil || size != item.Size || hex.EncodeToString(digest.Sum(nil)) != item.SHA256 { return ErrInvalidArchive }
		delete(wanted, name)
	}
	if len(wanted) != 0 { return ErrInvalidArchive }
	return nil
}

func extractAndVerify(entries []*zip.File, root string, manifest Manifest) error {
	wanted := map[string]File{}
	for _, item := range manifest.Files { wanted[item.Path] = item }
	for _, entry := range entries {
		name := filepath.ToSlash(entry.Name); if name == ManifestPath { continue }
		item, ok := wanted[name]; if !ok || entry.FileInfo().Mode().IsDir() { return ErrInvalidArchive }
		if item.Size > MaxArchiveBytes { return ErrInvalidArchive }
		path := filepath.Join(root, filepath.FromSlash(name)); if !within(root, path) { return ErrInvalidArchive }
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil { return err }
		input, err := entry.Open(); if err != nil { return ErrInvalidArchive }
		output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600); if err != nil { input.Close(); return err }
		digest := sha256.New(); size, copyErr := io.Copy(io.MultiWriter(output, digest), io.LimitReader(input, MaxArchiveBytes+1)); input.Close(); output.Close()
		if copyErr != nil || size != item.Size || size > MaxArchiveBytes || hex.EncodeToString(digest.Sum(nil)) != item.SHA256 { return ErrInvalidArchive }
		delete(wanted, name)
	}
	if len(wanted) != 0 { return ErrInvalidArchive }
	return nil
}

func digestFile(path string) (string, int64, error) {
	input, err := os.Open(path); if err != nil { return "", 0, err }; defer input.Close()
	digest := sha256.New(); size, err := io.Copy(digest, io.LimitReader(input, MaxArchiveBytes+1)); if err != nil { return "", 0, err }
	if size > MaxArchiveBytes { return "", 0, ErrInvalidArchive }
	return hex.EncodeToString(digest.Sum(nil)), size, nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path); return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
