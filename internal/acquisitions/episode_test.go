package acquisitions

import (
	"context"
	"testing"
	"time"

	"github.com/lekramon/piratarr/internal/series"
	"github.com/lekramon/piratarr/internal/settings"
	"github.com/lekramon/piratarr/internal/sources"
)

type episodeObservationRepository struct {
	Repository
	observations []Observation
}

func (r *episodeObservationRepository) SaveObservations(_ context.Context, values []Observation, _, _ int, _ string) error {
	r.observations = append(r.observations, values...)
	return nil
}

func TestRememberEpisodeCreatesDurableEpisodeObservations(t *testing.T) {
	repo := &episodeObservationRepository{}
	service := New(repo, nil)
	service.Now = func() time.Time { return time.Date(2026, 11, 11, 3, 0, 0, 0, time.UTC) }
	profile := settings.Settings{Revision: 5, Profile: settings.Profile{AcceptedResolutions: []string{"1080p"}}}
	result := sources.EpisodeSearchResult{SourceURL: "http://source.invalid/api", EpisodeID: 12, SourceName: "Indexer", SourceRevision: 8, ProfileRevision: 5, Items: []sources.Candidate{{ID: "candidate", Title: "Pilot S01E01", Status: "match", DownloadReference: "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567"}}}

	remembered, err := service.RememberEpisode(context.Background(), result, series.Episode{ID: 12, Title: "Pilot"}, profile)
	if err != nil || len(remembered.Items) != 1 || remembered.Items[0].ObservationID == "" || len(repo.observations) != 1 {
		t.Fatal(remembered, repo.observations, err)
	}
	observation := repo.observations[0]
	if observation.MovieID != 0 || observation.EpisodeID == nil || *observation.EpisodeID != 12 || observation.Reference == "" || observation.SourceRevision != 8 || observation.ProfileRevision != 5 {
		t.Fatal(observation)
	}
}
