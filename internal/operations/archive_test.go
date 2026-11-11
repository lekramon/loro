package operations

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofrs/flock"
)

func TestBackupCheckRestorePreservesPrivateFilesAndExcludesPayloads(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "piratarr.db"), []byte("database"), 0600); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(filepath.Join(root, "media"), 0700); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(root, "media", "movie.mkv"), []byte("media"), 0600); err != nil { t.Fatal(err) }
	archive := filepath.Join(t.TempDir(), "backup.zip")
	if err := Backup(root, archive); err != nil { t.Fatal(err) }
	manifest, err := Check(archive); if err != nil { t.Fatal(err) }
	if len(manifest.Files) != 1 || manifest.Files[0].Path != "piratarr.db" { t.Fatalf("unexpected manifest: %+v", manifest) }
	destination := filepath.Join(t.TempDir(), "restored")
	if err := Restore(archive, destination); err != nil { t.Fatal(err) }
	content, err := os.ReadFile(filepath.Join(destination, "piratarr.db")); if err != nil || string(content) != "database" { t.Fatalf("restored database: %v %q", err, content) }
	if _, err := os.Stat(filepath.Join(destination, "media")); !os.IsNotExist(err) { t.Fatal("media should not be restored") }
}

func TestCheckRejectsChangedArchivePayload(t *testing.T) {
	root := t.TempDir(); if err := os.WriteFile(filepath.Join(root, "piratarr.db"), []byte("database"), 0600); err != nil { t.Fatal(err) }
	archive := filepath.Join(t.TempDir(), "backup.zip"); if err := Backup(root, archive); err != nil { t.Fatal(err) }
	reader, err := zip.OpenReader(archive); if err != nil { t.Fatal(err) }
	manifestBytes, err := os.ReadFile(archive); if err != nil { t.Fatal(err) }; reader.Close()
	if len(manifestBytes) == 0 || strings.TrimSpace(string(manifestBytes)) == "" { t.Fatal("archive was empty") }
	// Rebuild a valid zip with the original manifest and a changed payload.
	old, err := zip.OpenReader(archive); if err != nil { t.Fatal(err) }; defer old.Close()
	changed := filepath.Join(t.TempDir(), "changed.zip"); output, err := os.Create(changed); if err != nil { t.Fatal(err) }
	writer := zip.NewWriter(output)
	for _, entry := range old.File { target, createErr := writer.Create(entry.Name); if createErr != nil { t.Fatal(createErr) }; input, openErr := entry.Open(); if openErr != nil { t.Fatal(openErr) }; data, readErr := io.ReadAll(input); input.Close(); if readErr != nil { t.Fatal(readErr) }; if entry.Name == "piratarr.db" { data = []byte("changed") }; if _, err = target.Write(data); err != nil { t.Fatal(err) } }
	if err = writer.Close(); err != nil { t.Fatal(err) }; if err = output.Close(); err != nil { t.Fatal(err) }
	if _, err = Check(changed); err != ErrInvalidArchive { t.Fatalf("expected checksum failure, got %v", err) }
}

func TestCheckRejectsTraversalEntry(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "bad.zip")
	file, err := os.Create(archive); if err != nil { t.Fatal(err) }
	writer := zip.NewWriter(file)
	entry, err := writer.Create("../escape"); if err != nil { t.Fatal(err) }
	if _, err = entry.Write([]byte("bad")); err != nil { t.Fatal(err) }
	if err = writer.Close(); err != nil { t.Fatal(err) }; if err = file.Close(); err != nil { t.Fatal(err) }
	if _, err = Check(archive); err != ErrInvalidArchive { t.Fatalf("expected invalid archive, got %v", err) }
}

func TestRestoreRefusesNonEmptyDestination(t *testing.T) {
	root := t.TempDir(); archive := filepath.Join(t.TempDir(), "backup.zip")
	if err := os.WriteFile(filepath.Join(root, "piratarr.db"), []byte("database"), 0600); err != nil { t.Fatal(err) }
	if err := Backup(root, archive); err != nil { t.Fatal(err) }
	destination := filepath.Join(t.TempDir(), "existing"); if err := os.Mkdir(destination, 0700); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(destination, "keep"), []byte("keep"), 0600); err != nil { t.Fatal(err) }
	if err := Restore(archive, destination); err == nil { t.Fatal("restore overwrote non-empty destination") }
}

func TestRestoreRefusesLockedDestination(t *testing.T) {
	root := t.TempDir(); if err := os.WriteFile(filepath.Join(root, "piratarr.db"), []byte("database"), 0600); err != nil { t.Fatal(err) }
	archive := filepath.Join(t.TempDir(), "backup.zip"); if err := Backup(root, archive); err != nil { t.Fatal(err) }
	destination := t.TempDir(); lock := flock.New(filepath.Join(destination, "piratarr.lock")); acquired, err := lock.TryLock(); if err != nil || !acquired { t.Fatal(err) }; defer lock.Unlock()
	if err = Restore(archive, destination); err == nil { t.Fatal("restore ignored an active data-directory lock") }
}
