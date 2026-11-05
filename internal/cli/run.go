package cli

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lekramon/piratarr/internal/application"
	"github.com/lekramon/piratarr/internal/bootstrap"
	"github.com/lekramon/piratarr/internal/healthcheck"
	"github.com/lekramon/piratarr/internal/webui"
)

func Main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("application stopped", "cause", err.Error())
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		return healthcheck.Run(context.Background(), os.Args[2:], os.Getenv)
	}
	config, err := bootstrap.Parse(os.Args[1:], os.Getenv)
	if err != nil {
		return err
	}
	startup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	app, err := application.Open(startup, config, webui.Assets())
	cancel()
	if err != nil {
		return err
	}
	stop, release := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer release()
	served := make(chan error, 1)
	go func() { served <- app.Serve() }()
	logger.Info("application ready", "address", app.Address(), "preview", true)
	select {
	case <-stop.Done():
	case err = <-served:
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
	}
	shutdown, closeCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer closeCancel()
	shutdownErr := app.Shutdown(shutdown)
	if err != nil {
		return err
	}
	return shutdownErr
}
