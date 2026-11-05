// Package application owns startup ordering and resource lifetime.
package application

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"time"

	"github.com/lekramon/piratarr/internal/bootstrap"
	"github.com/lekramon/piratarr/internal/httpapi"
	"github.com/lekramon/piratarr/internal/storage"
)

type Application struct {
	Server   *http.Server
	Handler  *httpapi.Handler
	listener net.Listener
	store    *storage.Store
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
	handler := httpapi.New(store, assets, listener.Addr().String())
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	return &Application{Server: server, Handler: handler, listener: listener, store: store}, nil
}

func (a *Application) Address() string { return a.listener.Addr().String() }
func (a *Application) Serve() error    { return a.Server.Serve(a.listener) }

func (a *Application) Shutdown(ctx context.Context) error {
	a.Handler.Stop()
	err := a.Server.Shutdown(ctx)
	if err != nil {
		_ = a.Server.Close()
	}
	// Also cover Open followed by Shutdown before Serve registers the listener.
	_ = a.listener.Close()
	closeErr := a.store.Close()
	if err != nil {
		return err
	}
	return closeErr
}
