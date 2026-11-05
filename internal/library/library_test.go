package library

import (
	"context"
	"errors"
	"github.com/lekramon/piratarr/internal/settings"
	"os"
	"path/filepath"
	"testing"
)

type repository struct {
	config           settings.Settings
	published        bool
	entered, release chan struct{}
}

func (r *repository) GetSettings(ctx context.Context) (settings.Settings, error) {
	if r.entered != nil {
		close(r.entered)
		select {
		case <-r.release:
		case <-ctx.Done():
			return settings.Settings{}, ctx.Err()
		}
	}
	return r.config, nil
}
func (r *repository) PublishInventory(context.Context, string, int, []Observation, int) error {
	r.published = true
	return nil
}
func (r *repository) ListInventory(context.Context, string, int, int, string) (Snapshot, error) {
	return Snapshot{}, nil
}
func TestLimitsAndCancellationNeverPublish(t *testing.T) {
	for _, kind := range []string{"files", "entries", "depth", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			os.WriteFile(filepath.Join(root, "a.mkv"), []byte("a"), 0600)
			os.WriteFile(filepath.Join(root, "b.mp4"), []byte("b"), 0600)
			os.Mkdir(filepath.Join(root, "nested"), 0700)
			repo := &repository{config: settings.Settings{Revision: 1, Directories: settings.Directories{MovieDirectory: root}}}
			service := New(repo, t.TempDir())
			switch kind {
			case "files":
				service.maxFiles = 1
			case "entries":
				service.maxEntries = 1
			case "depth":
				service.maxDepth = 0
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind == "cancel" {
				cancel()
			}
			_, err := service.Scan(ctx)
			if err == nil || repo.published {
				t.Fatal("partial scan published", err)
			}
			if kind != "cancel" && !errors.Is(err, ErrLimit) {
				t.Fatal(err)
			}
		})
	}
}
func TestConcurrentScanExclusion(t *testing.T) {
	repo := &repository{config: settings.Settings{Revision: 1, Directories: settings.Directories{MovieDirectory: t.TempDir()}}, entered: make(chan struct{}), release: make(chan struct{})}
	service := New(repo, t.TempDir())
	done := make(chan error, 1)
	go func() { _, err := service.Scan(context.Background()); done <- err }()
	<-repo.entered
	if _, err := service.Scan(context.Background()); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	close(repo.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
