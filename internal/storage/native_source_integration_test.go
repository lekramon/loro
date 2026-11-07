package storage

import (
	"context"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestNativeSourceMigrationPreservesConfigurationAndRejectsOlderReader(t *testing.T) {
	ctx := context.Background()
	v10 := fstest.MapFS{}
	names, _ := fs.Glob(migrations, "migrations/*.sql")
	for _, name := range names {
		if name < "migrations/00011_native_source.sql" {
			data, _ := fs.ReadFile(migrations, name)
			v10[name] = &fstest.MapFile{Data: data}
		}
	}
	directory := t.TempDir()
	old, err := open(ctx, directory, v10)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := old.CreateSettings(ctx, libraryInput(t))
	if err != nil {
		t.Fatal(err)
	}
	source, err := old.SaveSource(ctx, 0, "Old source", "http://localhost/api", "private-key")
	if err != nil {
		t.Fatal(err)
	}
	old.Close()
	store, err := Open(ctx, directory)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := store.GetSource(ctx)
	if err != nil || !reflect.DeepEqual(actual, source) || actual.Kind != "torznab" {
		t.Fatal("migration changed existing source", err)
	}
	current, err := store.GetSettings(ctx)
	if err != nil || !reflect.DeepEqual(current, settings) {
		t.Fatal("migration changed settings", err)
	}
	direct, err := store.SaveSource(ctx, source.Revision, "Direct", "http://localhost", "", "1337x")
	if err != nil || direct.Kind != "1337x" {
		t.Fatal(err)
	}
	store.Close()
	if older, err := open(ctx, directory, v10); err == nil {
		older.Close()
		t.Fatal("old reader accepted schema 11")
	}
	reopened, err := Open(ctx, directory)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	actual, err = reopened.GetSource(ctx)
	if err != nil || !reflect.DeepEqual(actual, direct) {
		t.Fatal("native source not persisted", err)
	}

	broken := fstest.MapFS{}
	for name, data := range v10 {
		broken[name] = data
	}
	broken["migrations/00011_native_source.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nALTER TABLE source_configuration ADD COLUMN kind TEXT;\nINSERT INTO absent VALUES(1);\n")}
	clean := t.TempDir()
	original, err := open(ctx, clean, v10)
	if err != nil {
		t.Fatal(err)
	}
	original.Close()
	if failed, err := open(ctx, clean, broken); err == nil {
		failed.Close()
		t.Fatal("failed migration accepted")
	}
	restored, err := open(ctx, clean, v10)
	if err != nil {
		t.Fatal("failed migration not atomic", err)
	}
	restored.Close()
}
