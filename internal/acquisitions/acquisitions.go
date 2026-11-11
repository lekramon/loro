// Package acquisitions persists manual choices before any external submission.
package acquisitions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"time"

	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/series"
	"github.com/lekramon/piratarr/internal/settings"
	"github.com/lekramon/piratarr/internal/sources"
)

type Error string

func (e Error) Error() string { return string(e) }

const (
	ErrUnavailable         Error = "storage-unavailable"
	ErrNotFound            Error = "acquisition-not-found"
	ErrObservationNotFound Error = "observation-not-found"
	ErrExpired             Error = "observation-expired"
	ErrObservationChanged  Error = "observation-changed"
	ErrConfirmation        Error = "profile-confirmation-required"
	ErrIdempotency         Error = "idempotency-conflict"
	ErrChanged             Error = "acquisition-changed"
	ErrState               Error = "acquisition-state-conflict"
	ErrBusy                Error = "preparation-in-progress"
	ErrValidation          Error = "validation-failed"
)

type Observation struct {
	ID               string
	MovieID          int64
	EpisodeID        *int64
	MovieTitle       string
	OriginalLanguage string
	SourceName       string
	SourceRevision   int
	ProfileRevision  int
	Candidate        sources.Candidate
	Profile          settings.Profile
	SourceURL        string `json:"-"`
	Reference        string `json:"-"`
	HashClaim        string `json:"-"`
	ReferenceIssue   string `json:"-"`
	ExpiresAt        string
}
type Submission struct {
	State          string  `json:"state"`
	ClientRevision int     `json:"clientRevision"`
	Attempts       int     `json:"attempts"`
	ErrorCode      *string `json:"errorCode"`
	UpdatedAt      string  `json:"updatedAt"`
}
type Acquisition struct {
	Submission        *Submission       `json:"submission,omitempty"`
	ID                string            `json:"id"`
	MovieID           int64             `json:"movieId"`
	EpisodeID         *int64            `json:"episodeId,omitempty"`
	MovieTitle        string            `json:"movieTitle"`
	OriginalLanguage  string            `json:"originalLanguage"`
	SourceName        string            `json:"sourceName"`
	SourceRevision    int               `json:"sourceRevision"`
	ProfileRevision   int               `json:"profileRevision"`
	Candidate         sources.Candidate `json:"candidate"`
	Profile           settings.Profile  `json:"profile"`
	OverrideConfirmed bool              `json:"overrideConfirmed"`
	State             string            `json:"state"`
	Revision          int               `json:"revision"`
	InfoHash          *string           `json:"infoHash"`
	Kind              *string           `json:"kind"`
	ReviewCode        *string           `json:"reviewCode"`
	CreatedAt         string            `json:"createdAt"`
	UpdatedAt         string            `json:"updatedAt"`
}
type Record struct {
	Acquisition
	SourceURL      string `json:"-"`
	Reference      string `json:"-"`
	HashClaim      string `json:"-"`
	ReferenceIssue string `json:"-"`
	Payload        []byte `json:"-"`
}
type Selection struct {
	ObservationID   string `json:"observationId"`
	IdempotencyKey  string `json:"idempotencyKey"`
	ConfirmOverride bool   `json:"confirmOverride"`
}
type List struct {
	Items  []Acquisition `json:"items"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}
type Preparation struct {
	Hash       string
	Kind       string
	ReviewCode string
	Payload    []byte
}
type Repository interface {
	SaveObservations(context.Context, []Observation, int, int, string) error
	SelectAcquisition(context.Context, Selection, string, string) (Acquisition, bool, error)
	GetAcquisition(context.Context, string) (Record, error)
	ListAcquisitions(context.Context, int, int, int64) (List, error)
	BeginPreparation(context.Context, string, int, string) (Record, error)
	FinishPreparation(context.Context, string, int, Preparation, string) (Acquisition, error)
	CancelAcquisition(context.Context, string, int, string) (Acquisition, error)
}
type ReferenceResolver interface {
	ResolveReference(context.Context, string, string) (string, string)
}
type Service struct {
	Repo     Repository
	Resolver ReferenceResolver
	Now      func() time.Time
	gate     chan struct{}
}

func New(repo Repository, resolver ReferenceResolver) *Service {
	return &Service{Repo: repo, Resolver: resolver, Now: time.Now, gate: make(chan struct{}, 1)}
}
func Identifier() string {
	var data [16]byte
	_, err := rand.Read(data[:])
	if err != nil {
		panic("random identifier unavailable")
	}
	return hex.EncodeToString(data[:])
}

var idPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,80}$`)

func ValidID(id string) bool { return idPattern.MatchString(id) }
func (s *Service) Remember(ctx context.Context, page sources.ReleasePage, movie catalog.Movie, profile settings.Settings) (sources.ReleasePage, error) {
	now := s.Now().UTC()
	observations := make([]Observation, 0, len(page.Items))
	for i := range page.Items {
		candidate := &page.Items[i]
		candidate.ObservationID = Identifier()
		observations = append(observations, Observation{ID: candidate.ObservationID, MovieID: movie.ID, MovieTitle: movie.Title, OriginalLanguage: movie.OriginalLanguage, SourceName: page.SourceName, SourceRevision: page.SourceRevision, ProfileRevision: profile.Revision, Candidate: *candidate, Profile: profile.Profile, SourceURL: page.SourceURL, Reference: candidate.DownloadReference, HashClaim: candidate.HashClaim, ReferenceIssue: candidate.ReferenceIssue, ExpiresAt: now.Add(30 * time.Minute).Format(time.RFC3339Nano)})
	}
	if err := s.Repo.SaveObservations(ctx, observations, page.SourceRevision, profile.Revision, now.Format(time.RFC3339Nano)); err != nil {
		return sources.ReleasePage{}, err
	}
	return page, nil
}

func (s *Service) RememberEpisode(ctx context.Context, result sources.EpisodeSearchResult, episode series.Episode, profile settings.Settings) (sources.EpisodeSearchResult, error) {
	now := s.Now().UTC()
	observations := make([]Observation, 0, len(result.Items))
	for i := range result.Items {
		candidate := &result.Items[i]
		candidate.ObservationID = Identifier()
		episodeID := episode.ID
		observations = append(observations, Observation{ID: candidate.ObservationID, EpisodeID: &episodeID, MovieTitle: episode.Title, SourceName: result.SourceName, SourceRevision: result.SourceRevision, ProfileRevision: profile.Revision, Candidate: *candidate, Profile: profile.Profile, SourceURL: result.SourceURL, Reference: candidate.DownloadReference, HashClaim: candidate.HashClaim, ReferenceIssue: candidate.ReferenceIssue, ExpiresAt: now.Add(30 * time.Minute).Format(time.RFC3339Nano)})
	}
	if err := s.Repo.SaveObservations(ctx, observations, result.SourceRevision, profile.Revision, now.Format(time.RFC3339Nano)); err != nil {
		return sources.EpisodeSearchResult{}, err
	}
	return result, nil
}
func (s *Service) Select(ctx context.Context, input Selection) (Acquisition, bool, error) {
	if !ValidID(input.ObservationID) || !keyPattern.MatchString(input.IdempotencyKey) {
		return Acquisition{}, false, ErrValidation
	}
	return s.Repo.SelectAcquisition(ctx, input, Identifier(), s.Now().UTC().Format(time.RFC3339Nano))
}
func (s *Service) Prepare(ctx context.Context, id string, revision int) (Acquisition, error) {
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	default:
		return Acquisition{}, ErrBusy
	}
	record, err := s.Repo.BeginPreparation(ctx, id, revision, s.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return Acquisition{}, err
	}
	if record.State != "preparing" {
		return record.Acquisition, nil
	}
	result := resolve(ctx, record, s.Resolver)
	// A disconnected browser must not strand a completed local decision; a crash is recovered at startup.
	deadline := time.Now().Add(5 * time.Second)
	finish, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	return s.Repo.FinishPreparation(finish, id, record.Revision, result, s.Now().UTC().Format(time.RFC3339Nano))
}
