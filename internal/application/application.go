// Package application owns startup ordering and resource lifetime.
package application

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/lekramon/piratarr/internal/bootstrap"
	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/downloads"
	"github.com/lekramon/piratarr/internal/httpapi"
	"github.com/lekramon/piratarr/internal/jobs"
	"github.com/lekramon/piratarr/internal/storage"
	"github.com/lekramon/piratarr/internal/submissions"
	"github.com/lekramon/piratarr/internal/tmdb"
)

type Application struct {
	stopWorkers context.CancelFunc
	workers     sync.WaitGroup
	Server      *http.Server
	Handler     *httpapi.Handler
	listener    net.Listener
	store       *storage.Store
}

func Open(ctx context.Context, config bootstrap.Config, assets fs.FS) (*Application, error) {
	directory, err := bootstrap.PrepareDataDir(config.DataDir)
	if err != nil {
		return nil, err
	}
	store, err := storage.Open(ctx, directory)
	if err != nil {
		return nil, err
	}
	// Storage and migrations must succeed before opening the HTTP port.
	listener, err := net.Listen("tcp", config.Listen)
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("open HTTP listener: %w", err)
	}
	handler := httpapi.New(store, assets, listener.Addr().String(), catalog.New(store, tmdb.New(tmdb.Config{TokenFile: config.TMDBTokenFile, LocalTokenFile: filepath.Join(directory, "credentials", "tmdb-read-token")}), directory))
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	workerCtx, stop := context.WithCancel(handler.BackgroundContext(context.Background()))
	app := &Application{Server: server, Handler: handler, listener: listener, store: store, stopWorkers: stop}
	count := config.Workers
	if count < 1 || count > 8 {
		count = 4
	}
	app.workers.Add(1)
	go func() {
		defer app.workers.Done()
		jobs.Run(workerCtx, count, (&submissions.Engine{Repo: store}).Step, (&downloads.Engine{Repo: store}).Step)
	}()
	return app, nil
}

func (a *Application) Address() string { return a.listener.Addr().String() }
func (a *Application) Serve() error    { return a.Server.Serve(a.listener) }

func (a *Application) Shutdown(ctx context.Context) error {
	a.Handler.Stop()
	a.stopWorkers()
	err := a.Server.Shutdown(ctx)
	if err != nil {
		_ = a.Server.Close()
	}
	// Also cover Open followed by Shutdown before Serve registers the listener.
	_ = a.listener.Close()
	a.workers.Wait()
	closeErr := a.store.Close()
	if err != nil {
		return err
	}
	return closeErr
}
