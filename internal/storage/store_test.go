package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestFreshStorageRestartAndConnectionPragmas(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	s, err := Open(ctx, directory)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := s.SetupCompleted(ctx)
	if err != nil || completed {
		t.Fatalf("fresh setup: %v, %v", completed, err)
	}
	if err := s.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	// Hold multiple real connections to ensure every new connection receives
	// the required pragmas, not only the first connection opened.
	var connections []*sql.Conn
	for range 4 {
		connection, err := s.read.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		connections = append(connections, connection)
		for pragma, expected := range map[string]int{"foreign_keys": 1, "busy_timeout": 5000} {
			var value int
			if err := connection.QueryRowContext(ctx, "PRAGMA "+pragma).Scan(&value); err != nil || value != expected {
				t.Errorf("%s: value=%d err=%v", pragma, value, err)
			}
		}
		var mode string
		if err := connection.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
			t.Errorf("journal mode %q: %v", mode, err)
		}
	}
	for _, connection := range connections {
		_ = connection.Close()
	}
	// A sentinel persisted in an unrelated table proves restart does not reset
	// the database, without implementing the next ticket's setup mutation.
	if _, err := s.write.ExecContext(ctx, "CREATE TABLE restart_evidence (value TEXT); INSERT INTO restart_evidence VALUES ('preserved')"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, directory)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var value string
	if err := s.read.QueryRowContext(ctx, "SELECT value FROM restart_evidence").Scan(&value); err != nil || value != "preserved" {
		t.Fatalf("persistence: %q %v", value, err)
	}
}

func TestUnsupportedHistoryIsNotModified(t *testing.T) {
	directory := t.TempDir()
	s, err := Open(context.Background(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.write.Exec("INSERT INTO goose_db_version(version_id,is_applied) VALUES(999,true)"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), directory); err == nil {
		t.Fatal("accepted future database")
	}
	db, err := sql.Open("sqlite", filepath.Join(directory, "piratarr.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT count(*) FROM goose_db_version WHERE version_id=999 AND is_applied=true").Scan(&count); err != nil || count != 1 {
		t.Fatalf("history changed: %d %v", count, err)
	}
}

func TestFailedMigrationRollsBackAndReleasesLock(t *testing.T) {
	directory := t.TempDir()
	broken := fstest.MapFS{"migrations/00001_failure.sql": &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE should_rollback (id INTEGER);\nTHIS IS INVALID SQL;\n-- +goose Down\nDROP TABLE should_rollback;\n")}}
	if _, err := open(context.Background(), directory, broken); err == nil {
		t.Fatal("invalid migration accepted")
	}
	s, err := Open(context.Background(), directory)
	if err != nil {
		t.Fatalf("failed startup retained lock: %v", err)
	}
	defer s.Close()
	var count int
	if err := s.read.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='should_rollback'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("migration partially persisted: %d %v", count, err)
	}
}

func TestInvalidStorageLocation(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), file); err == nil {
		t.Fatal("accepted non-directory")
	}
	value, _ := os.ReadFile(file)
	if string(value) != "preserve" {
		t.Fatal("modified existing file")
	}
}

func TestKnownVersionWithMissingSchemaRefusesStartup(t *testing.T) {
	directory := t.TempDir()
	s, err := Open(context.Background(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.write.Exec("DROP TABLE audio_preferences"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), directory); err == nil {
		t.Fatal("started with missing required schema")
	}
}
