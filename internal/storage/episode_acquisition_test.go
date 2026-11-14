package storage

import (
	"context"
	"testing"
	"time"

	"github.com/lekramon/piratarr/internal/acquisitions"
	"github.com/lekramon/piratarr/internal/sources"
)

func TestEpisodeObservationCanBeSelectedWithoutMovieIdentity(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	profile, err := store.CreateSettings(ctx, libraryInput(t))
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.SaveSource(ctx, 0, "Source", "http://localhost:8081/api", "dummy-source")
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.write.ExecContext(ctx, `INSERT INTO series(provider,provider_id,title,metadata,added_at) VALUES('tmdb',1,'Series','{}','now')`)
	if err != nil {
		t.Fatal(err)
	}
	seriesID, _ := result.LastInsertId()
	result, err = store.write.ExecContext(ctx, `INSERT INTO seasons(series_id,season_number,metadata) VALUES(?,1,'{}')`, seriesID)
	if err != nil {
		t.Fatal(err)
	}
	seasonID, _ := result.LastInsertId()
	result, err = store.write.ExecContext(ctx, `INSERT INTO episodes(season_id,episode_number,title,metadata) VALUES(?,1,'Pilot','{}')`, seasonID)
	if err != nil {
		t.Fatal(err)
	}
	episodeID, _ := result.LastInsertId()
	now := time.Now().UTC()
	observationID := acquisitions.Identifier()
	magnet := "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567"
	observation := acquisitions.Observation{ID: observationID, EpisodeID: &episodeID, SourceRevision: source.Revision, ProfileRevision: profile.Revision, Candidate: sources.Candidate{ID: "episode-candidate", Status: "match"}, Reference: magnet, ExpiresAt: now.Add(time.Hour).Format(time.RFC3339Nano)}
	if err = store.SaveObservations(ctx, []acquisitions.Observation{observation}, source.Revision, profile.Revision, now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	selected, created, err := store.SelectAcquisition(ctx, acquisitions.Selection{ObservationID: observationID, IdempotencyKey: "episode-selection-key", ConfirmOverride: false}, acquisitions.Identifier(), now.Format(time.RFC3339Nano))
	if err != nil || !created || selected.MovieID != 0 || selected.EpisodeID == nil || *selected.EpisodeID != episodeID {
		t.Fatal(selected, created, err)
	}
	service := acquisitions.New(store, nil)
	ready, err := service.Prepare(ctx, selected.ID, selected.Revision)
	if err != nil || ready.State != "ready" || ready.InfoHash == nil || *ready.InfoHash != "0123456789abcdef0123456789abcdef01234567" {
		t.Fatal(ready, err)
	}
	client, err := store.SaveDownloadClient(ctx, 0, "http://localhost:8082", "admin", "dummy-client")
	if err != nil {
		t.Fatal(err)
	}
	queued, err := store.EnqueueSubmission(ctx, ready.ID, ready.Revision, client.Revision, profile.Revision, false)
	if err != nil || queued.Submission == nil || queued.Submission.State != "queued" || queued.EpisodeID == nil || *queued.EpisodeID != episodeID {
		t.Fatal(queued, err)
	}
}
