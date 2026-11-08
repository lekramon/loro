package storage

import (
	"context"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/lekramon/piratarr/internal/sources"
)

func TestMultipleSourcesMigrationPreservesActiveSourceAndRevisions(t *testing.T) {
	ctx := context.Background()
	v11 := fstest.MapFS{}
	names, _ := fs.Glob(migrations, "migrations/*.sql")
	for _, name := range names {
		if name < "migrations/00012_multiple_sources.sql" {
			data, _ := fs.ReadFile(migrations, name)
			v11[name] = &fstest.MapFile{Data: data}
		}
	}
	directory := t.TempDir()
	old, err := open(ctx, directory, v11)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = old.CreateSettings(ctx, libraryInput(t)); err != nil {
		t.Fatal(err)
	}
	previous, err := old.SaveSource(ctx, 0, "Torrentio", "http://localhost:9696/1/api", "private-key")
	if err != nil {
		t.Fatal(err)
	}
	caps := sources.Capabilities{Search: sources.SearchCapability{Available: true, SupportedParams: []string{"q"}}, Categories: []sources.Category{}}
	if err = old.SaveSourceCapabilities(ctx, previous.Revision, caps, "2026-11-07T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	previous, _ = old.GetSource(ctx)
	old.Close()

	store, err := Open(ctx, directory)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := store.GetSource(ctx)
	if err != nil || !reflect.DeepEqual(actual, previous) || !actual.Active || actual.ID != 1 || actual.APIKey != "private-key" {
		t.Fatal("migration changed active source", err, actual)
	}
	created, err := store.CreateSource(ctx, "Pirate Bay", "http://localhost:9696/6/api", "private-key", "torznab")
	if err != nil || created.Active || created.Revision <= actual.Revision {
		t.Fatal("inactive source not created with global revision", err, created)
	}
	if _, err = store.CreateSource(ctx, "Duplicate", created.URL, "private-key", "torznab"); err != sources.ErrExists {
		t.Fatal("duplicate endpoint accepted", err)
	}
	activated, err := store.ActivateSource(ctx, created.ID, created.Revision)
	if err != nil || !activated.Active || activated.Revision <= created.Revision {
		t.Fatal("source not activated with new global revision", err, activated)
	}
	if _, err = store.ActivateSource(ctx, actual.ID, actual.Revision+1); err != sources.ErrChanged {
		t.Fatal("stale activation accepted", err)
	}
	store.Close()
	if older, err := open(ctx, directory, v11); err == nil {
		older.Close()
		t.Fatal("old reader accepted schema 12")
	}
	reopened, err := Open(ctx, directory)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	current, err := reopened.GetSource(ctx)
	list, listErr := reopened.ListSources(ctx)
	if err != nil || listErr != nil || !reflect.DeepEqual(current, activated) || len(list) != 2 {
		t.Fatal("multiple sources not preserved across restart", err, listErr, current, list)
	}
}

func TestMultipleSourcesMigrationFailureIsAtomic(t *testing.T) {
	ctx := context.Background()
	v11 := fstest.MapFS{}
	names, _ := fs.Glob(migrations, "migrations/*.sql")
	for _, name := range names {
		if name < "migrations/00012_multiple_sources.sql" {
			data, _ := fs.ReadFile(migrations, name)
			v11[name] = &fstest.MapFile{Data: data}
		}
	}
	broken := fstest.MapFS{}
	for name, data := range v11 {
		broken[name] = data
	}
	broken["migrations/00012_multiple_sources.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nALTER TABLE source_configuration ADD COLUMN active INTEGER;\nINSERT INTO absent VALUES(1);\n")}
	directory := t.TempDir()
	old, err := open(ctx, directory, v11)
	if err != nil {
		t.Fatal(err)
	}
	old.Close()
	if failed, err := open(ctx, directory, broken); err == nil {
		failed.Close()
		t.Fatal("failed migration accepted")
	}
	restored, err := open(ctx, directory, v11)
	if err != nil {
		t.Fatal("failed migration was not atomic", err)
	}
	restored.Close()
}
