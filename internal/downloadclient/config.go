// Package downloadclient owns the private qBittorrent connection and its checks.
package downloadclient

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

var (
	ErrUnavailable  = errors.New("download client storage unavailable")
	ErrChanged      = errors.New("download client changed")
	ErrValidation   = errors.New("invalid download client configuration")
	ErrUnconfigured = errors.New("download client unconfigured")
	ErrBusy         = errors.New("download client test in progress")
)

type Public struct {
	Configured  bool    `json:"configured"`
	Revision    int     `json:"revision"`
	URL         string  `json:"url"`
	Username    string  `json:"username"`
	HasPassword bool    `json:"hasPassword"`
	AppVersion  *string `json:"appVersion"`
	APIVersion  *string `json:"apiVersion"`
	CheckedAt   *string `json:"checkedAt"`
}
type Config struct {
	Public
	Password string `json:"-"`
}
type Input struct {
	Revision int    `json:"revision"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
}
type TestResult struct {
	Connected      bool   `json:"connected"`
	ClientRevision int    `json:"clientRevision"`
	AppVersion     string `json:"appVersion"`
	APIVersion     string `json:"apiVersion"`
	CheckedAt      string `json:"checkedAt"`
}
type Repository interface {
	GetDownloadClient(context.Context) (Config, error)
	SaveDownloadClient(context.Context, int, string, string, string) (Config, error)
	SaveDownloadClientTest(context.Context, int, string, string, string) error
}
type Service struct {
	repo             Repository
	gate             chan struct{}
	cooldown         time.Time // Owned by the test gate; Save never accesses these fields.
	cooldownRevision int
}

func New(repo Repository) *Service { return &Service{repo: repo, gate: make(chan struct{}, 1)} }
func (s *Service) Get(ctx context.Context) (Public, error) {
	config, err := s.repo.GetDownloadClient(ctx)
	return config.Public, err
}
func (s *Service) Save(ctx context.Context, input Input) (Public, error) {
	input.URL = strings.TrimRight(strings.TrimSpace(input.URL), "/")
	input.Username = strings.TrimSpace(input.Username)
	if input.Revision < 0 || !utf8.ValidString(input.Username) || utf8.RuneCountInString(input.Username) < 1 || utf8.RuneCountInString(input.Username) > 100 || !utf8.ValidString(input.Password) || len(input.Password) > 1024 || strings.ContainsRune(input.Password, 0) || len(input.URL) > 2048 || strings.Contains(input.URL, "#") {
		return Public{}, ErrValidation
	}
	for _, r := range input.Username {
		if unicode.IsControl(r) {
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
	current, err := s.repo.GetDownloadClient(ctx)
	if err != nil {
		return Public{}, err
	}
	if current.Revision != input.Revision {
		return Public{}, ErrChanged
	}
	password := input.Password
	if password == "" {
		if !current.Configured || current.URL != input.URL || current.Username != input.Username {
			return Public{}, ErrValidation
		}
		password = current.Password
	}
	saved, err := s.repo.SaveDownloadClient(ctx, input.Revision, input.URL, input.Username, password)
	return saved.Public, err
}
func (s *Service) Test(ctx context.Context) (TestResult, error) {
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	default:
		return TestResult{}, ErrBusy
	}
	config, err := s.repo.GetDownloadClient(ctx)
	if err != nil {
		return TestResult{}, err
	}
	if !config.Configured {
		return TestResult{}, ErrUnconfigured
	}
	if remaining := time.Until(s.cooldown); s.cooldownRevision == config.Revision && remaining > 0 {
		return TestResult{}, &ProviderError{Code: "download-client-rate-limited", RetryAfter: int(remaining.Seconds()) + 1}
	}
	app, api, err := testConnection(ctx, config)
	if err != nil {
		var failure *ProviderError
		if errors.As(err, &failure) && failure.Code == "download-client-rate-limited" {
			s.cooldownRevision = config.Revision
			s.cooldown = time.Now().Add(time.Duration(failure.RetryAfter) * time.Second)
		}
		return TestResult{}, err
	}
	at := time.Now().UTC().Format(time.RFC3339Nano)
	if err = s.repo.SaveDownloadClientTest(ctx, config.Revision, app, api, at); err != nil {
		return TestResult{}, err
	}
	return TestResult{Connected: true, ClientRevision: config.Revision, AppVersion: app, APIVersion: api, CheckedAt: at}, nil
}
