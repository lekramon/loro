package downloads

import (
	"context"
	"errors"
	"github.com/lekramon/piratarr/internal/acquisitions"
	"github.com/lekramon/piratarr/internal/downloadclient"
	"time"
)

type Engine struct {
	Repo Repository
	Now  func() time.Time
}

func (e *Engine) now() time.Time {
	if e.Now != nil {
		return e.Now().UTC()
	}
	return time.Now().UTC()
}
func (e *Engine) Step(parent context.Context) (bool, error) {
	job, err := e.Repo.ClaimDownloadPoll(parent, acquisitions.Identifier(), e.now())
	if err != nil || job == nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		tick := time.NewTicker(15 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if e.Repo.RenewDownloadPoll(ctx, *job, e.now()) != nil {
					cancel()
					return
				}
			}
		}
	}()
	result := e.observe(ctx, *job)
	cancel()
	<-stopped
	finish, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	return true, e.Repo.FinishDownloadPoll(finish, *job, result, e.now())
}
func (e *Engine) observe(ctx context.Context, j Job) Result {
	config, err := e.Repo.GetDownloadClient(ctx)
	if err != nil {
		return failure(ErrUnavailable)
	}
	if config.Revision != j.ClientRevision {
		return failure(Error("download-client-changed"))
	}
	if err = e.Repo.RenewDownloadPoll(ctx, j, e.now()); err != nil {
		return failure(err)
	}
	result := Result{}
	err = downloadclient.WithSession(ctx, config, func(s *downloadclient.Session, _, _ string) error {
		if err := e.Repo.RenewDownloadPoll(ctx, j, e.now()); err != nil {
			return err
		}
		var err error
		result, err = readTransfer(ctx, s, j, e.now())
		return err
	})
	if err != nil {
		return failure(err)
	}
	return result
}
func failure(err error) Result {
	r := Result{State: "needs-review", Code: "storage-unavailable"}
	var provider *downloadclient.ProviderError
	var domain Error
	if errors.As(err, &provider) {
		r.Code = provider.Code
		r.RetryAfter = provider.RetryAfter
		switch provider.Code {
		case "download-client-timeout", "download-client-unavailable", "download-client-rate-limited":
			r.State = "unavailable"
		}
	}
	if errors.As(err, &domain) {
		r.Code = string(domain)
	}
	return r
}
