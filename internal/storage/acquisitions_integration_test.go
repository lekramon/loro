package storage

import (
	"context"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lekramon/piratarr/internal/acquisitions"
	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/sources"
)

func TestAcquisitionMigrationRollbackPreservationAndStartupRecovery(t *testing.T) {
	ctx := context.Background()
	data := t.TempDir()
	v7 := fstest.MapFS{}
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if name >= "migrations/00008_acquisitions.sql" {
			continue
		}
		bytes, err := fs.ReadFile(migrations, name)
		if err != nil {
			t.Fatal(err)
		}
		v7[name] = &fstest.MapFile{Data: bytes}
	}
	old, err := open(ctx, data, v7)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := old.CreateSettings(ctx, libraryInput(t))
	if err != nil {
		t.Fatal(err)
	}
	movie, _, err := old.RegisterMovie(ctx, catalog.Metadata{TMDBID: 42, Title: "Preserved catalog", Genres: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	source, err := old.SaveSource(ctx, 0, "Source", "http://localhost:8081/api", "dummy-source-key")
	if err != nil {
		t.Fatal(err)
	}
	client, err := old.SaveDownloadClient(ctx, 0, "http://localhost:8082", "admin", "dummy-client-password")
	if err != nil {
		t.Fatal(err)
	}
	old.Close()
	faulty := fstest.MapFS{}
	for name, file := range v7 {
		faulty[name] = file
	}
	faulty["migrations/00008_acquisitions.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE release_observations(id TEXT);\nINSERT INTO missing_table VALUES(1);\n")}
	if bad, err := open(ctx, data, faulty); err == nil {
		bad.Close()
		t.Fatal("migration failure accepted")
	}
	old, err = open(ctx, data, v7)
	if err != nil {
		t.Fatal("old state not readable after rollback", err)
	}
	old.Close()
	store, err := Open(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if actual, err := store.GetSettings(ctx); err != nil || !reflect.DeepEqual(actual, settings) {
		t.Fatal("settings changed", err)
	}
	if actual, err := store.GetSource(ctx); err != nil || !reflect.DeepEqual(actual, source) {
		t.Fatal("source changed", err)
	}
	if actual, err := store.GetDownloadClient(ctx); err != nil || !reflect.DeepEqual(actual, client) {
		t.Fatal("client changed", err)
	}
	now := time.Now().UTC()
	observations := []acquisitions.Observation{}
	for _, id := range []string{"11111111111111111111111111111111", "22222222222222222222222222222222", "33333333333333333333333333333333"} {
		observations = append(observations, acquisitions.Observation{ID: id, MovieID: movie.ID, MovieTitle: movie.Title, SourceName: source.Name, SourceRevision: source.Revision, ProfileRevision: settings.Revision, Profile: settings.Profile, Candidate: sources.Candidate{ID: id, ObservationID: id, Title: "Saved candidate", Status: "unknown", Checks: []sources.ProfileCheck{}}, SourceURL: source.URL, Reference: "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&tr=private", ExpiresAt: now.Add(30 * time.Minute).Format(time.RFC3339Nano)})
	}
	if err = store.SaveObservations(ctx, observations, source.Revision, settings.Revision, now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err = store.write.Exec(`CREATE TRIGGER fail_acquisition_request BEFORE INSERT ON acquisition_requests BEGIN SELECT RAISE(ABORT,'private request failure'); END;`); err != nil {
		t.Fatal(err)
	}
	input := acquisitions.Selection{ObservationID: observations[0].ID, IdempotencyKey: "migration-request-key-001", ConfirmOverride: true}
	if _, _, err = store.SelectAcquisition(ctx, input, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", now.Format(time.RFC3339Nano)); err != acquisitions.ErrUnavailable {
		t.Fatal("partial selection committed", err)
	}
	page, err := store.ListAcquisitions(ctx, 25, 0, 0)
	if err != nil || page.Total != 0 {
		t.Fatal("failed selection left intent", err)
	}
	store.write.Exec("DROP TRIGGER fail_acquisition_request")
	selected, _, err := store.SelectAcquisition(ctx, input, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", now.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	preparing, err := store.BeginPreparation(ctx, selected.ID, selected.Revision, now.Format(time.RFC3339Nano))
	if err != nil || preparing.State != "preparing" {
		t.Fatal(preparing, err)
	}
	readyInput := acquisitions.Selection{ObservationID: observations[1].ID, IdempotencyKey: "migration-request-key-002", ConfirmOverride: true}
	ready, _, err := store.SelectAcquisition(ctx, readyInput, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", now.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	running, err := store.BeginPreparation(ctx, ready.ID, ready.Revision, now.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	ready, err = store.FinishPreparation(ctx, ready.ID, running.Revision, acquisitions.Preparation{Hash: "0123456789abcdef0123456789abcdef01234567", Kind: "torrent", Payload: []byte("private-metainfo-bytes")}, now.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	third, _, err := store.SelectAcquisition(ctx, acquisitions.Selection{ObservationID: observations[2].ID, IdempotencyKey: "migration-request-key-003", ConfirmOverride: true}, "cccccccccccccccccccccccccccccccc", now.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
	if rejected, err := open(ctx, data, v7); err == nil {
		rejected.Close()
		t.Fatal("v7 reader accepted schema 8")
	}
	restarted, err := Open(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	recovered, err := restarted.GetAcquisition(ctx, preparing.ID)
	if err != nil || recovered.State != "needs-review" || recovered.ReviewCode == nil || *recovered.ReviewCode != "preparation-interrupted" || recovered.Revision != preparing.Revision+1 {
		t.Fatal(recovered, err)
	}
	prepared, err := restarted.GetAcquisition(ctx, ready.ID)
	if err != nil || prepared.State != "ready" || string(prepared.Payload) != "private-metainfo-bytes" || prepared.Revision != ready.Revision {
		t.Fatal("ready payload changed", err)
	}
	untouched, err := restarted.GetAcquisition(ctx, third.ID)
	if err != nil || untouched.State != "selected" {
		t.Fatal("selected intent replayed", err)
	}
	if _, _, err = restarted.SelectAcquisition(ctx, input, "dddddddddddddddddddddddddddddddd", now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal("request mapping did not survive", err)
	}
	if _, err = restarted.write.Exec("DROP TABLE acquisition_requests"); err != nil {
		t.Fatal(err)
	}
	if restarted.Ready(ctx) == nil {
		t.Fatal("missing acquisition table considered ready")
	}
}
