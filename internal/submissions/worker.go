// Package submissions executes durable intent with conservative uncertain-add recovery.
package submissions

import (
	"context"
	"errors"
	"github.com/lekramon/piratarr/internal/communications"
	"time"

	"github.com/lekramon/piratarr/internal/acquisitions"
	"github.com/lekramon/piratarr/internal/downloadclient"
	"github.com/lekramon/piratarr/internal/jobs"
)

type Job struct {
	ID, Hash, Directory, Token                 string
	ClientRevision, SettingsRevision, Attempts int
	Attempted                                  bool
}
type Outcome struct {
	State, Code string
	Delay       int
}
type Repository interface {
	EnqueueSubmission(context.Context, string, int, int, int, bool) (acquisitions.Acquisition, error)
	ClaimSubmission(context.Context, string, time.Time) (*Job, error)
	RenewSubmission(context.Context, Job, time.Time) error
	MarkSubmissionAttempt(context.Context, Job, time.Time) error
	FinishSubmission(context.Context, Job, Outcome, time.Time) error
	GetAcquisition(context.Context, string) (acquisitions.Record, error)
	GetDownloadClient(context.Context) (downloadclient.Config, error)
}
type Engine struct{ Repo Repository }

// Run owns workers until cancellation, including any in-flight bounded operation.
func (e *Engine) Run(ctx context.Context, count int) { jobs.Run(ctx, count, e.Step) }

// Step is the same claim/effect/finish path used by production workers and API simulator checks.
func (e *Engine) Step(parent context.Context) (bool, error) {
	job, err := e.Repo.ClaimSubmission(parent, acquisitions.Identifier(), time.Now().UTC())
	if err != nil || job == nil {
		return false, err
	}
	parent = communications.Correlate(parent)
	start := time.Now()
	communications.Emit(parent, communications.Event{Integration: "jobs", Operation: "jobs.submission", Phase: "operation.started"})
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()
	renewed := make(chan struct{})
	go func() {
		defer close(renewed)
		tick := time.NewTicker(15 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if e.Repo.RenewSubmission(ctx, *job, time.Now().UTC()) != nil {
					cancel()
					return
				}
			}
		}
	}()
	outcome := e.execute(ctx, *job)
	cancel()
	<-renewed
	finish, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	err = e.Repo.FinishSubmission(finish, *job, outcome, time.Now().UTC())
	event := communications.Event{Integration: "jobs", Operation: "jobs.submission", Phase: "operation.completed", ElapsedMS: communications.Ptr(time.Since(start).Milliseconds())}
	if outcome.Code != "" {
		event.Phase = "operation.failed"
		event.Level = "warning"
		event.Code = &outcome.Code
	}
	if err != nil {
		event.Phase = "operation.failed"
		event.Level = "warning"
		event.Code = communications.Ptr("storage-unavailable")
	}
	communications.Emit(ctx, event)
	return true, err
}
func (e *Engine) execute(ctx context.Context, job Job) Outcome {
	result := Outcome{State: "needs-review"}
	config, err := e.Repo.GetDownloadClient(ctx)
	if err != nil {
		return Outcome{State: "needs-review", Code: "storage-unavailable"}
	}
	if config.Revision != job.ClientRevision {
		return Outcome{State: "needs-review", Code: "download-client-changed"}
	}
	record, err := e.Repo.GetAcquisition(ctx, job.ID)
	if err != nil {
		return Outcome{State: "needs-review", Code: "storage-unavailable"}
	}
	if err = e.Repo.RenewSubmission(ctx, job, time.Now().UTC()); err != nil {
		var d acquisitions.Error
		if errors.As(err, &d) {
			return Outcome{State: "needs-review", Code: string(d)}
		}
		return Outcome{State: "needs-review", Code: "storage-unavailable"}
	}
	attempted := job.Attempted
	err = downloadclient.WithSession(ctx, config, func(session *downloadclient.Session, _, _ string) error {
		// This also checks the lease and local revision fence before any client work.
		if err := e.Repo.RenewSubmission(ctx, job, time.Now().UTC()); err != nil {
			return err
		}
		status, err := observe(ctx, session, job)
		if err != nil {
			return err
		}
		if status != "absent" {
			result = Outcome{State: status, Code: ownershipCode(status)}
			return nil
		}
		if attempted {
			result = Outcome{State: "needs-review", Code: "submission-uncertain"}
			return nil
		}
		if record.Kind == nil || (*record.Kind != "magnet" && *record.Kind != "torrent") || (*record.Kind == "torrent" && len(record.Payload) == 0) {
			return acquisitions.Error("submission-payload-invalid")
		}
		if err = ensureCategory(ctx, session, job.Directory); err != nil {
			return err
		}
		if err = e.Repo.MarkSubmissionAttempt(ctx, job, time.Now().UTC()); err != nil {
			return err
		}
		attempted = true
		// Persisted evidence deliberately precedes the request, even if the process dies here.
		addErr := add(ctx, session, job, record)
		status, err = observe(ctx, session, job)
		if err == nil && status == "succeeded" {
			result = Outcome{State: "succeeded"}
			return nil
		}
		if err == nil && status != "absent" {
			result = Outcome{State: "needs-review", Code: ownershipCode(status)}
			return nil
		}
		if addErr != nil {
			return addErr
		}
		if err != nil {
			return err
		}
		result = Outcome{State: "needs-review", Code: "submission-uncertain"}
		return nil
	})
	if err != nil {
		code := "submission-failed"
		var provider *downloadclient.ProviderError
		var domain acquisitions.Error
		if errors.As(err, &provider) {
			code = provider.Code
			result.Delay = provider.RetryAfter
		}
		if errors.As(err, &domain) {
			code = string(domain)
		}
		if attempted && code != "download-client-changed" && code != "submission-configuration-changed" {
			code = "submission-uncertain"
		}
		result.State = "needs-review"
		result.Code = code
	}
	return result
}
func ownershipCode(status string) string {
	if status == "succeeded" {
		return ""
	}
	return "submission-ownership-conflict"
}
