// Package sources configures the local installation's Torznab integration.
package sources

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var ErrUnavailable = errors.New("source storage unavailable")
var ErrChanged = errors.New("source changed")
var ErrValidation = errors.New("invalid source configuration")
var ErrUnconfigured = errors.New("source unconfigured")
var ErrBusy = errors.New("source test in progress")

type SearchCapability struct {
	Available       bool     `json:"available"`
	SupportedParams []string `json:"supportedParams"`
}
type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
type Capabilities struct {
	Search       SearchCapability `json:"search"`
	MovieSearch  SearchCapability `json:"movieSearch"`
	MaxLimit     *int             `json:"maxLimit"`
	DefaultLimit *int             `json:"defaultLimit"`
	Categories   []Category       `json:"categories"`
}
type Public struct {
	Configured   bool          `json:"configured"`
	Revision     int           `json:"revision"`
	Name         string        `json:"name"`
	URL          string        `json:"url"`
	HasAPIKey    bool          `json:"hasApiKey"`
	Capabilities *Capabilities `json:"capabilities"`
	CheckedAt    *string       `json:"checkedAt"`
}
type Config struct {
	Public
	APIKey string `json:"-"`
}
type Input struct {
	Revision    int    `json:"revision"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	APIKey      string `json:"apiKey,omitempty"`
	ClearAPIKey bool   `json:"clearApiKey,omitempty"`
}
type TestResult struct {
	Connected      bool         `json:"connected"`
	SourceRevision int          `json:"sourceRevision"`
	Capabilities   Capabilities `json:"capabilities"`
	CheckedAt      string       `json:"checkedAt"`
}
type Repository interface {
	GetSource(context.Context) (Config, error)
	SaveSource(context.Context, int, string, string, string) (Config, error)
	SaveSourceCapabilities(context.Context, int, Capabilities, string) error
}
type Service struct {
	repo   Repository
	client *Client
	gate   chan struct{}
}

func New(repo Repository) *Service {
	return &Service{repo: repo, client: NewClient(), gate: make(chan struct{}, 1)}
}
func (s *Service) Get(ctx context.Context) (Public, error) {
	config, err := s.repo.GetSource(ctx)
	return config.Public, err
}
func (s *Service) Save(ctx context.Context, input Input) (Public, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.URL = strings.TrimSpace(input.URL)
	input.APIKey = strings.TrimSpace(input.APIKey)
	if input.Revision < 0 || !utf8.ValidString(input.Name) || utf8.RuneCountInString(input.Name) < 1 || utf8.RuneCountInString(input.Name) > 100 || len(input.URL) > 2048 || strings.Contains(input.URL, "#") || len(input.APIKey) > 1024 || input.ClearAPIKey && input.APIKey != "" {
		return Public{}, ErrValidation
	}
	for _, r := range input.Name {
		if unicode.IsControl(r) {
			return Public{}, ErrValidation
		}
	}
	for _, r := range input.APIKey {
		if r < 33 || r > 126 {
			return Public{}, ErrValidation
		}
	}
	endpoint, err := url.Parse(input.URL)
	if err != nil || endpoint == nil || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.ForceQuery || endpoint.Fragment != "" || endpoint.Opaque != "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || strings.HasSuffix(endpoint.Host, ":") {
		return Public{}, ErrValidation
	}
	if endpoint.Port() != "" {
		port, err := strconv.Atoi(endpoint.Port())
		if err != nil || port < 1 || port > 65535 {
			return Public{}, ErrValidation
		}
	}
	current, err := s.repo.GetSource(ctx)
	if err != nil {
		return Public{}, err
	}
	if current.Revision != input.Revision {
		return Public{}, ErrChanged
	}
	if current.Configured && current.URL != input.URL && input.APIKey == "" && !input.ClearAPIKey {
		return Public{}, ErrValidation
	}
	key := current.APIKey
	if input.APIKey != "" {
		key = input.APIKey
	}
	if input.ClearAPIKey {
		key = ""
	}
	saved, err := s.repo.SaveSource(ctx, input.Revision, input.Name, input.URL, key)
	return saved.Public, err
}
func (s *Service) Test(ctx context.Context) (TestResult, error) {
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	default:
		return TestResult{}, ErrBusy
	}
	config, err := s.repo.GetSource(ctx)
	if err != nil {
		return TestResult{}, err
	}
	if !config.Configured {
		return TestResult{}, ErrUnconfigured
	}
	caps, err := s.client.Capabilities(ctx, config)
	if err != nil {
		return TestResult{}, err
	}
	at := time.Now().UTC().Format(time.RFC3339Nano)
	if err = s.repo.SaveSourceCapabilities(ctx, config.Revision, caps, at); err != nil {
		return TestResult{}, err
	}
	return TestResult{Connected: true, SourceRevision: config.Revision, Capabilities: caps, CheckedAt: at}, nil
}
