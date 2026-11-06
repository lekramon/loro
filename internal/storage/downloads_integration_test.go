package storage

import (
	"context"
	"github.com/lekramon/piratarr/internal/acquisitions"
	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/downloads"
	"github.com/lekramon/piratarr/internal/sources"
	"github.com/lekramon/piratarr/internal/submissions"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
	"time"
)

func TestDownloadMigrationBackfillAtomicHistoryLeasesAndRecovery(t *testing.T) {
	ctx := context.Background()
	data := t.TempDir()
	v9 := fstest.MapFS{}
	names, _ := fs.Glob(migrations, "migrations/*.sql")
	for _, name := range names {
		if name >= "migrations/00010_download_tracking.sql" {
			continue
		}
		body, _ := fs.ReadFile(migrations, name)
		v9[name] = &fstest.MapFile{Data: body}
	}
	old, err := open(ctx, data, v9)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := old.CreateSettings(ctx, libraryInput(t))
	if err != nil {
		t.Fatal(err)
	}
	movie, _, err := old.RegisterMovie(ctx, catalog.Metadata{TMDBID: 42, Title: "Preserved movie", Genres: []string{}})
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
	stamp := now.Format(time.RFC3339Nano)
	obs := acquisitions.Observation{ID: acquisitions.Identifier(), MovieID: movie.ID, MovieTitle: movie.Title, SourceRevision: source.Revision, ProfileRevision: cfg.Revision, Candidate: sources.Candidate{ID: "preserved-candidate", Title: "Movie.release", Status: "unknown"}, ExpiresAt: now.Add(time.Hour).Format(time.RFC3339Nano), Reference: "private-magnet", SourceURL: source.URL}
	if err = old.SaveObservations(ctx, []acquisitions.Observation{obs}, source.Revision, cfg.Revision, stamp); err != nil {
		t.Fatal(err)
	}
	selection := acquisitions.Selection{ObservationID: obs.ID, IdempotencyKey: "download-migration-key", ConfirmOverride: true}
	a, _, err := old.SelectAcquisition(ctx, selection, acquisitions.Identifier(), stamp)
	if err != nil {
		t.Fatal(err)
	}
	preparing, err := old.BeginPreparation(ctx, a.ID, a.Revision, stamp)
	if err != nil {
		t.Fatal(err)
	}
	a, err = old.FinishPreparation(ctx, a.ID, preparing.Revision, acquisitions.Preparation{Hash: "0123456789abcdef0123456789abcdef01234567", Kind: "torrent", Payload: []byte("private metainfo")}, stamp)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = old.EnqueueSubmission(ctx, a.ID, a.Revision, client.Revision, cfg.Revision, false); err != nil {
		t.Fatal(err)
	}
	job, err := old.ClaimSubmission(ctx, "before-upgrade", time.Now())
	if err != nil || job == nil {
		t.Fatal(job, err)
	}
	if err = old.MarkSubmissionAttempt(ctx, *job, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err = old.FinishSubmission(ctx, *job, submissions.Outcome{State: "succeeded"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	// Keep a second add-attempted submission unconfirmed across the upgrade.
	other := obs
	other.ID = acquisitions.Identifier()
	other.Candidate.ID = "pending-candidate"
	if err = old.SaveObservations(ctx, []acquisitions.Observation{other}, source.Revision, cfg.Revision, stamp); err != nil {
		t.Fatal(err)
	}
	b, _, err := old.SelectAcquisition(ctx, acquisitions.Selection{ObservationID: other.ID, IdempotencyKey: "pending-upgrade-key", ConfirmOverride: true}, acquisitions.Identifier(), stamp)
	if err != nil {
		t.Fatal(err)
	}
	bp, err := old.BeginPreparation(ctx, b.ID, b.Revision, stamp)
	if err != nil {
		t.Fatal(err)
	}
	b, err = old.FinishPreparation(ctx, b.ID, bp.Revision, acquisitions.Preparation{Hash: "ffffffffffffffffffffffffffffffffffffffff", Kind: "torrent", Payload: []byte("private pending")}, stamp)
	if err != nil {
		t.Fatal(err)
	}
	b, err = old.EnqueueSubmission(ctx, b.ID, b.Revision, client.Revision, cfg.Revision, false)
	if err != nil {
		t.Fatal(err)
	}
	unfinished, err := old.ClaimSubmission(ctx, "unconfirmed-before-upgrade", time.Now())
	if err != nil || unfinished == nil {
		t.Fatal(err)
	}
	if err = old.MarkSubmissionAttempt(ctx, *unfinished, time.Now()); err != nil {
		t.Fatal(err)
	}
	preserved, _ := old.GetAcquisition(ctx, a.ID)
	credential := filepath.Join(data, "dummy-credential")
	os.WriteFile(credential, []byte("dummy-token"), 0600)
	old.Close()
	broken := fstest.MapFS{}
	for name, file := range v9 {
		broken[name] = file
	}
	broken["migrations/00010_download_tracking.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE download_tasks(id TEXT);\nINSERT INTO absent VALUES(1);\n")}
	if store, err := open(ctx, data, broken); err == nil {
		store.Close()
		t.Fatal("failed migration accepted")
	}
	old, err = open(ctx, data, v9)
	if err != nil {
		t.Fatal("v9 unreadable after rollback", err)
	}
	old.Close()
	store, err := Open(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { store.Close() }()
	if current, err := store.GetAcquisition(ctx, a.ID); err != nil || !reflect.DeepEqual(current, preserved) {
		t.Fatal("acquisition/private payload changed", err)
	}
	if current, err := store.GetSettings(ctx); err != nil || !reflect.DeepEqual(current, cfg) {
		t.Fatal("configuration changed", err)
	}
	if current, err := store.GetSource(ctx); err != nil || !reflect.DeepEqual(current, source) {
		t.Fatal("source changed", err)
	}
	if current, err := store.GetDownloadClient(ctx); err != nil || !reflect.DeepEqual(current, client) {
		t.Fatal("client changed", err)
	}
	if _, _, err = store.SelectAcquisition(ctx, selection, acquisitions.Identifier(), stamp); err != nil {
		t.Fatal("replay mapping lost", err)
	}
	listing, err := store.ListDownloads(ctx, 25, 0, 0)
	if err != nil || listing.Total != 1 {
		t.Fatal("unconfirmed submission was backfilled", listing, err)
	}
	initial, err := store.GetDownload(ctx, a.ID)
	if err != nil || initial.Download.State != "pending" || initial.EventTotal != 1 || initial.Download.Progress != nil {
		t.Fatal("backfill", initial, err)
	}
	// Event failure must roll back the task update and leave the current lease usable.
	lease, err := store.ClaimDownloadPoll(ctx, "first-poll", time.Now())
	if err != nil || lease == nil {
		t.Fatal(lease, err)
	}
	if other, err := store.ClaimDownloadPoll(ctx, "other", time.Now()); err != nil || other != nil {
		t.Fatal("overlapping poll", other, err)
	}
	store.write.Exec(`CREATE TRIGGER fail_download_event BEFORE INSERT ON download_events BEGIN SELECT RAISE(ABORT,'event failure'); END`)
	progress := 1.0
	size := int64(100)
	zero := int64(0)
	state := "uploading"
	out := downloads.Result{State: "completed", Metrics: &downloads.Metrics{Progress: &progress, SizeBytes: &size, RemainingBytes: &zero, DownloadSpeed: &zero, ClientState: &state}, CompletedAt: &stamp, Handoff: &downloads.Handoff{Version: 1, State: "awaiting-validation", File: &downloads.File{Index: 0, RelativePath: "Movie/movie.mkv", SizeBytes: 100}}}
	if err = store.FinishDownloadPoll(ctx, *lease, out, time.Now()); err != downloads.ErrUnavailable {
		t.Fatal("event failure committed", err)
	}
	unchanged, _ := store.GetDownload(ctx, a.ID)
	if unchanged.Download.Revision != 1 || unchanged.EventTotal != 1 || unchanged.Download.Handoff != nil || unchanged.Download.PollState != "running" {
		t.Fatal("partial result", unchanged)
	}
	store.write.Exec("DROP TRIGGER fail_download_event")
	if err = store.FinishDownloadPoll(ctx, *lease, out, time.Now()); err != nil {
		t.Fatal(err)
	}
	completed, _ := store.GetDownload(ctx, a.ID)
	if completed.Download.Handoff == nil || completed.EventTotal != 2 {
		t.Fatal(completed)
	}
	queued, err := store.QueueDownloadCheck(ctx, a.ID, completed.Download.Revision)
	if err != nil || queued.Handoff != nil || queued.CompletedAt != nil {
		t.Fatal("queue retained handoff", queued, err)
	}
	start := time.Now().UTC()
	running, err := store.ClaimDownloadPoll(ctx, "running", start)
	if err != nil || running == nil {
		t.Fatal(running, err)
	}
	forged := *running
	forged.Token = "stale"
	if err = store.RenewDownloadPoll(ctx, forged, start); err != downloads.ErrChanged {
		t.Fatal("stale renewal", err)
	}
	if err = store.RenewDownloadPoll(ctx, *running, start.Add(15*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err = store.RenewDownloadPoll(ctx, *running, start.Add(65*time.Second)); err != nil {
		t.Fatal("renewal did not extend lease", err)
	}
	expired := start.Add(126 * time.Second)
	if err = store.FinishDownloadPoll(ctx, *running, out, expired); err != downloads.ErrChanged {
		t.Fatal("expired finish", err)
	}
	if other, err := store.ClaimDownloadPoll(ctx, "no-takeover", expired); err != nil || other != nil {
		t.Fatal("live expired work reassigned")
	}
	store.Close()
	if prior, err := open(ctx, data, v9); err == nil {
		prior.Close()
		t.Fatal("old reader accepted v10")
	}
	store, err = Open(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	recovered, _ := store.GetDownload(ctx, a.ID)
	if recovered.Download.PollState != "queued" || recovered.Download.Handoff != nil || recovered.Download.Progress == nil {
		t.Fatal("recovery", recovered)
	}
	fresh, err := store.ClaimDownloadPoll(ctx, "new-process", time.Now())
	if err != nil || fresh == nil {
		t.Fatal(err)
	}
	if err = store.FinishDownloadPoll(ctx, *running, out, time.Now()); err != downloads.ErrChanged {
		t.Fatal("stale process finish", err)
	}
	finishedAt := time.Now().UTC()
	if err = store.FinishDownloadPoll(ctx, *fresh, downloads.Result{State: "unavailable", Code: "download-client-rate-limited", RetryAfter: 60}, finishedAt); err != nil {
		t.Fatal(err)
	}
	unavailable, _ := store.GetDownload(ctx, a.ID)
	next, _ := time.Parse(time.RFC3339Nano, *unavailable.Download.NextCheckAt)
	if next.Before(finishedAt.Add(59*time.Second)) || unavailable.Download.Handoff != nil || *unavailable.Download.Progress != 1 {
		t.Fatal("lost cooldown/last metrics", unavailable)
	}
	if _, err = store.QueueDownloadCheck(ctx, a.ID, unavailable.Download.Revision); err != downloads.ErrCooldown {
		t.Fatal("cooldown bypass", err)
	}
	for range 60 {
		if _, err = store.write.Exec("INSERT INTO download_events(acquisition_id,state,created_at) VALUES(?,'paused',?)", a.ID, stamp); err != nil {
			t.Fatal(err)
		}
	}
	history, _ := store.GetDownload(ctx, a.ID)
	if len(history.Events) != 50 || history.EventTotal <= 50 {
		t.Fatal("unbounded/lost history", history.EventTotal, len(history.Events))
	}
	token, _ := os.ReadFile(credential)
	if string(token) != "dummy-token" {
		t.Fatal("credential changed")
	}
	// Failure to create tracking must also roll back submission confirmation.
	resumed, err := store.ClaimSubmission(ctx, "resume-unconfirmed", time.Now())
	if err != nil || resumed == nil || !resumed.Attempted {
		t.Fatal(resumed, err)
	}
	store.write.Exec(`CREATE TRIGGER fail_download_task BEFORE INSERT ON download_tasks BEGIN SELECT RAISE(ABORT,'task failure'); END`)
	if err = store.FinishSubmission(ctx, *resumed, submissions.Outcome{State: "succeeded"}, time.Now()); err != acquisitions.ErrUnavailable {
		t.Fatal("partial submission confirmation", err)
	}
	saved, _ := store.GetAcquisition(ctx, b.ID)
	if saved.Revision != b.Revision || saved.Submission.State != "running" {
		t.Fatal("confirmation not atomic", saved)
	}
	if _, err = store.GetDownload(ctx, b.ID); err != downloads.ErrNotFound {
		t.Fatal("unconfirmed task appeared", err)
	}
	store.write.Exec("DROP TRIGGER fail_download_task")
	if err = store.FinishSubmission(ctx, *resumed, submissions.Outcome{State: "succeeded"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	created, err := store.GetDownload(ctx, b.ID)
	if err != nil || created.Download.State != "pending" || created.EventTotal != 1 {
		t.Fatal("tracking creation", created, err)
	}
	if _, err = store.write.Exec("ALTER TABLE download_tasks DROP COLUMN observed_at"); err != nil {
		t.Fatal(err)
	}
	if store.Ready(ctx) == nil {
		t.Fatal("missing required download column is ready")
	}
}
