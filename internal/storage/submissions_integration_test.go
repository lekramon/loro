package storage

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lekramon/piratarr/internal/acquisitions"
	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/sources"
	"github.com/lekramon/piratarr/internal/submissions"
)

func TestSubmissionMigrationLeaseFencesAndRecovery(t *testing.T) {
	ctx := context.Background()
	data := t.TempDir()
	v8 := fstest.MapFS{}
	names, _ := fs.Glob(migrations, "migrations/*.sql")
	for _, name := range names {
		if name >= "migrations/00009_submission_jobs.sql" {
			continue
		}
		body, _ := fs.ReadFile(migrations, name)
		v8[name] = &fstest.MapFile{Data: body}
	}
	old, err := open(ctx, data, v8)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := old.CreateSettings(ctx, libraryInput(t))
	if err != nil {
		t.Fatal(err)
	}
	movie, _, err := old.RegisterMovie(ctx, catalog.Metadata{TMDBID: 42, Title: "Preserved", Genres: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	source, err := old.SaveSource(ctx, 0, "Source", "http://localhost:8081/api", "dummy-source")
	if err != nil {
		t.Fatal(err)
	}
	client, err := old.SaveDownloadClient(ctx, 0, "http://localhost:8082", "admin", "dummy-client")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	obs := acquisitions.Observation{ID: acquisitions.Identifier(), MovieID: movie.ID, SourceRevision: source.Revision, ProfileRevision: profile.Revision, Candidate: sources.Candidate{ID: "candidate", Status: "unknown"}, ExpiresAt: now.Add(time.Hour).Format(time.RFC3339Nano), Reference: "private-magnet", SourceURL: source.URL}
	if err = old.SaveObservations(ctx, []acquisitions.Observation{obs}, source.Revision, profile.Revision, now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	selection := acquisitions.Selection{ObservationID: obs.ID, IdempotencyKey: "preserved-selection", ConfirmOverride: true}
	a, _, err := old.SelectAcquisition(ctx, selection, acquisitions.Identifier(), now.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	prep, err := old.BeginPreparation(ctx, a.ID, a.Revision, now.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	a, err = old.FinishPreparation(ctx, a.ID, prep.Revision, acquisitions.Preparation{Hash: "0123456789abcdef0123456789abcdef01234567", Kind: "torrent", Payload: []byte("private payload")}, now.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	credential := filepath.Join(data, "dummy-credential")
	if err = os.WriteFile(credential, []byte("dummy-token"), 0600); err != nil {
		t.Fatal(err)
	}
	old.Close()
	faulty := fstest.MapFS{}
	for name, file := range v8 {
		faulty[name] = file
	}
	faulty["migrations/00009_submission_jobs.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE submission_jobs(id TEXT);\nINSERT INTO missing_table VALUES(1);\n")}
	if broken, err := open(ctx, data, faulty); err == nil {
		broken.Close()
		t.Fatal("failed migration accepted")
	}
	old, err = open(ctx, data, v8)
	if err != nil {
		t.Fatal("v8 unreadable after rollback", err)
	}
	old.Close()
	store, err := Open(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { store.Close() }()
	if got, e := store.GetSettings(ctx); e != nil || !reflect.DeepEqual(got, profile) {
		t.Fatal("settings changed", e)
	}
	if got, e := store.GetSource(ctx); e != nil || !reflect.DeepEqual(got, source) {
		t.Fatal("source changed", e)
	}
	if got, e := store.GetDownloadClient(ctx); e != nil || !reflect.DeepEqual(got, client) {
		t.Fatal("client changed", e)
	}
	if got, e := store.GetAcquisition(ctx, a.ID); e != nil || string(got.Payload) != "private payload" || !reflect.DeepEqual(got.Acquisition, a) {
		t.Fatal("intent changed", e)
	}
	if _, _, e := store.SelectAcquisition(ctx, selection, acquisitions.Identifier(), now.Format(time.RFC3339Nano)); e != nil {
		t.Fatal("replay mapping lost", e)
	}
	store.write.Exec(`CREATE TRIGGER fail_submission BEFORE INSERT ON submission_jobs BEGIN SELECT RAISE(ABORT,'test'); END`)
	if _, e := store.EnqueueSubmission(ctx, a.ID, a.Revision, client.Revision, profile.Revision, false); e != acquisitions.ErrUnavailable {
		t.Fatal("partial queue", e)
	}
	before, _ := store.GetAcquisition(ctx, a.ID)
	if before.Revision != a.Revision || before.Submission != nil {
		t.Fatal("failed queue changed intent")
	}
	store.write.Exec("DROP TRIGGER fail_submission")
	var wg sync.WaitGroup
	out := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := store.EnqueueSubmission(ctx, a.ID, a.Revision, client.Revision, profile.Revision, false)
			out <- e
		}()
	}
	wg.Wait()
	close(out)
	successes := 0
	for e := range out {
		if e == nil {
			successes++
		} else if e != acquisitions.ErrChanged {
			t.Fatal(e)
		}
	}
	if successes != 1 {
		t.Fatal("concurrent queue exclusion", successes)
	}
	start := time.Now().UTC()
	job, e := store.ClaimSubmission(ctx, "lease-one", start)
	if e != nil || job == nil {
		t.Fatal(job, e)
	}
	if other, e := store.ClaimSubmission(ctx, "lease-two", start); e != nil || other != nil {
		t.Fatal("duplicate claim", other, e)
	}
	forged := *job
	forged.Token = "stale"
	if e = store.RenewSubmission(ctx, forged, start); e != acquisitions.ErrChanged {
		t.Fatal("stale renewal", e)
	}
	if e = store.MarkSubmissionAttempt(ctx, forged, start); e != acquisitions.ErrChanged {
		t.Fatal("stale add permission", e)
	}
	if e = store.RenewSubmission(ctx, *job, start.Add(15*time.Second)); e != nil {
		t.Fatal(e)
	}
	if e = store.MarkSubmissionAttempt(ctx, *job, start.Add(16*time.Second)); e != nil {
		t.Fatal(e)
	}
	// Ownership must survive past the original 60-second lease after renewal.
	if e = store.RenewSubmission(ctx, *job, start.Add(65*time.Second)); e != nil {
		t.Fatal("renewal did not extend original lease", e)
	}
	expired := start.Add(126 * time.Second)
	if e = store.FinishSubmission(ctx, *job, submissions.Outcome{State: "succeeded"}, expired); e != acquisitions.ErrChanged {
		t.Fatal("expired finish", e)
	}
	if other, e := store.ClaimSubmission(ctx, "no-live-takeover", expired); e != nil || other != nil {
		t.Fatal("expired live work reassigned", other, e)
	}
	store.Close()
	if prior, e := open(ctx, data, v8); e == nil {
		prior.Close()
		t.Fatal("v8 reader accepted v9")
	}
	store, err = Open(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	restored, _ := store.GetAcquisition(ctx, a.ID)
	if restored.Submission == nil || restored.Submission.State != "queued" {
		t.Fatal(restored)
	}
	reclaimed, e := store.ClaimSubmission(ctx, "new-process", time.Now())
	if e != nil || reclaimed == nil || !reclaimed.Attempted || reclaimed.Attempts != 2 {
		t.Fatal("lost add evidence", reclaimed, e)
	}
	if e = store.FinishSubmission(ctx, *job, submissions.Outcome{State: "succeeded"}, time.Now()); e != acquisitions.ErrChanged {
		t.Fatal("old process committed", e)
	}
	if e = store.FinishSubmission(ctx, *reclaimed, submissions.Outcome{State: "needs-review", Code: "submission-uncertain"}, time.Now()); e != nil {
		t.Fatal(e)
	}
	current, _ := store.GetAcquisition(ctx, a.ID)
	if current.Submission.State != "uncertain" || !errors.Is(cancelSubmissionTest(store, ctx, a.ID, current.Revision), acquisitions.Error("submission-exists")) {
		t.Fatal(current)
	}
	bytes, _ := os.ReadFile(credential)
	if string(bytes) != "dummy-token" {
		t.Fatal("credential touched")
	}
	if _, e = store.write.Exec("DROP TABLE submission_jobs"); e != nil {
		t.Fatal(e)
	}
	if store.Ready(ctx) == nil {
		t.Fatal("missing job table ready")
	}
}
func cancelSubmissionTest(s *Store, ctx context.Context, id string, revision int) error {
	_, err := s.CancelAcquisition(ctx, id, revision, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
