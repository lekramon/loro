package operations

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
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
