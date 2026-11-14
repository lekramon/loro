// Package downloads observes owned transfers and records evidence without importing files.
package downloads

import (
	"context"
	"github.com/lekramon/piratarr/internal/downloadclient"
	"time"
)

type Error string

func (e Error) Error() string { return string(e) }

const (
	ErrUnavailable Error = "storage-unavailable"
	ErrNotFound    Error = "download-not-found"
	ErrChanged     Error = "download-changed"
	ErrCooldown    Error = "download-check-cooldown"
	ErrValidation  Error = "validation-failed"
)

type Metrics struct {
	Progress       *float64 `json:"progress"`
	SizeBytes      *int64   `json:"sizeBytes"`
	RemainingBytes *int64   `json:"remainingBytes"`
	DownloadSpeed  *int64   `json:"downloadSpeed"`
	ClientState    *string  `json:"clientState"`
}
type File struct {
	Index        int    `json:"index"`
	RelativePath string `json:"relativePath"`
	SizeBytes    int64  `json:"sizeBytes"`
}
type Handoff struct {
	Version    int     `json:"version"`
	State      string  `json:"state"`
	ReasonCode *string `json:"reasonCode"`
	File       *File   `json:"file"`
}
type Task struct {
	Metrics
	AcquisitionID  string   `json:"acquisitionId"`
	MovieID        int64    `json:"movieId,omitempty"`
	EpisodeID      *int64   `json:"episodeId,omitempty"`
	MovieTitle     string   `json:"movieTitle"`
	ReleaseTitle   string   `json:"releaseTitle"`
	InfoHash       string   `json:"infoHash"`
	ClientRevision int      `json:"clientRevision"`
	Revision       int      `json:"revision"`
	State          string   `json:"state"`
	PollState      string   `json:"pollState"`
	NextCheckAt    *string  `json:"nextCheckAt"`
	ErrorCode      *string  `json:"errorCode"`
	LastCheckedAt  *string  `json:"lastCheckedAt"`
	LastObservedAt *string  `json:"lastObservedAt"`
	CompletedAt    *string  `json:"completedAt"`
	Handoff        *Handoff `json:"handoff"`
	UpdatedAt      string   `json:"updatedAt"`
}
type Event struct {
	State        string   `json:"state"`
	Code         *string  `json:"code"`
	Progress     *float64 `json:"progress"`
	HandoffState *string  `json:"handoffState"`
	CreatedAt    string   `json:"createdAt"`
}
type Detail struct {
	Download   Task    `json:"download"`
	Events     []Event `json:"events"`
	EventTotal int     `json:"eventTotal"`
}
type List struct {
	Items  []Task `json:"items"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}
type Job struct {
	ID, Hash, Directory, Token       string
	ClientRevision, SettingsRevision int
}
type Result struct {
	State, Code string
	RetryAfter  int
	Metrics     *Metrics
	CompletedAt *string
	Handoff     *Handoff
}
type Repository interface {
	ListDownloads(context.Context, int, int, int64) (List, error)
	GetDownload(context.Context, string) (Detail, error)
	QueueDownloadCheck(context.Context, string, int) (Task, error)
	ClaimDownloadPoll(context.Context, string, time.Time) (*Job, error)
	RenewDownloadPoll(context.Context, Job, time.Time) error
	FinishDownloadPoll(context.Context, Job, Result, time.Time) error
	GetDownloadClient(context.Context) (downloadclient.Config, error)
}
