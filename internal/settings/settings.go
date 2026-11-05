// Package settings defines the default profile and first-run setup behavior.
package settings

import (
	"context"
	"errors"
)

type Directories struct {
	DownloadDirectory string `json:"downloadDirectory"`
	MovieDirectory    string `json:"movieDirectory"`
}

type Profile struct {
	AcceptedResolutions []string `json:"acceptedResolutions"`
	PreferredResolution string   `json:"preferredResolution"`
	AudioPreferences    []string `json:"audioPreferences"`
}

type Input struct {
	Directories Directories `json:"directories"`
	Profile     Profile     `json:"profile"`
}

type Settings struct {
	Revision    int         `json:"revision"`
	Directories Directories `json:"directories"`
	Profile     Profile     `json:"profile"`
	UpdatedAt   string      `json:"updatedAt"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationError struct{ Fields []FieldError }

func (e *ValidationError) Error() string { return "Settings validation failed." }

var (
	ErrAlreadyCompleted = errors.New("setup already completed")
	ErrSetupRequired    = errors.New("setup required")
	ErrUnavailable      = errors.New("settings storage unavailable")
)

type Repository interface {
	SetupCompleted(context.Context) (bool, error)
	CreateSettings(context.Context, Input) (Settings, error)
	GetSettings(context.Context) (Settings, error)
}

type Service struct {
	Repository    Repository
	DataDirectory string
}

func (s Service) CompleteSetup(ctx context.Context, input Input) (Settings, error) {
	completed, err := s.Repository.SetupCompleted(ctx)
	if err != nil {
		return Settings{}, ErrUnavailable
	}
	if completed {
		return Settings{}, ErrAlreadyCompleted
	}
	validated, err := Validate(ctx, input, s.DataDirectory)
	if err != nil {
		return Settings{}, err
	}
	return s.Repository.CreateSettings(ctx, validated)
}
