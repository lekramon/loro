package downloadclient

import (
	"bytes"
	"context"
	"errors"
	"github.com/lekramon/piratarr/internal/communications"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ProviderError struct {
	Code       string
	RetryAfter int
}

func (e *ProviderError) Error() string { return e.Code }
func failed(code string) error         { return &ProviderError{Code: "download-client-" + code} }

type Session struct {
	client *http.Client
	base   *url.URL
}

func testConnection(ctx context.Context, config Config) (app, api string, err error) {
	err = WithSession(ctx, config, func(_ *Session, a, v string) error { app, api = a, v; return nil })
	return
}

// WithSession authenticates and verifies versions, then logs out best effort.
func WithSession(ctx context.Context, config Config, work func(*Session, string, string) error) error {
	jar, _ := cookiejar.New(nil)
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext, TLSHandshakeTimeout: 2 * time.Second, ResponseHeaderTimeout: 2 * time.Second, MaxIdleConns: 1, MaxIdleConnsPerHost: 1}
	defer transport.CloseIdleConnections()
	base, err := url.Parse(config.URL)
	if err != nil {
		return failed("invalid-response")
	}
	s := Session{client: &http.Client{Transport: communications.Transport{Base: transport, Integration: "download-client"}, Jar: jar, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, base: base}
	login, err := s.request(ctx, "POST", "auth/login", url.Values{"username": {config.Username}, "password": {config.Password}}.Encode())
	// Even an unexpected login body may have established a session.
	hasSID := false
	for _, cookie := range jar.Cookies(s.endpoint("app/version")) {
		if cookie.Name == "SID" && cookie.Value != "" {
			hasSID = true
		}
	}
	if hasSID {
		defer func() {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
			defer cancel()
			_, _ = s.request(cleanup, "POST", "auth/logout", "")
		}()
	}
	if err != nil {
		return err
	}
	if login == "Fails." {
		return failed("authentication-failed")
	}
	if login != "Ok." || !hasSID {
		return failed("invalid-response")
	}
	app, err := s.request(ctx, "GET", "app/version", "")
	if err != nil {
		return err
	}
	api, err := s.request(ctx, "GET", "app/webapiVersion", "")
	if err != nil {
		return err
	}
	appMatch := appPattern.FindStringSubmatch(app)
	apiMatch := apiPattern.FindStringSubmatch(api)
	if appMatch == nil || apiMatch == nil {
		return failed("invalid-response")
	}
	if appMatch[1] != "5" || apiMatch[1] != "2" {
		return failed("unsupported-version")
	}
	return work(&s, app, api)
}

var appPattern = regexp.MustCompile(`^v?([0-9]{1,6})\.[0-9]{1,6}\.[0-9]{1,6}$`)
var apiPattern = regexp.MustCompile(`^([0-9]{1,6})\.[0-9]{1,6}(?:\.[0-9]{1,6})?$`)

func (s Session) endpoint(route string) *url.URL {
	result := *s.base
	result.RawPath = strings.TrimRight(result.EscapedPath(), "/") + "/api/v2/" + route
	result.Path, _ = url.PathUnescape(result.RawPath) // Parsed base and fixed API route are already valid.
	return &result
}
func (s Session) request(ctx context.Context, method, route, body string) (string, error) {
	data, err := s.Exchange(ctx, method, route, "application/x-www-form-urlencoded", []byte(body), 4096)
	return strings.TrimSpace(string(data)), err
}

// Exchange permits fixed adapter routes and bounds response bytes. No redirects/retries.
func (s Session) Exchange(ctx context.Context, method, route, contentType string, body []byte, limit int64) ([]byte, error) {
	path, query, _ := strings.Cut(route, "?")
	target := s.endpoint(path)
	target.RawQuery = query
	req, err := http.NewRequestWithContext(ctx, method, target.String(), bytes.NewReader(body))
	if err != nil {
		return nil, failed("invalid-response")
	}
	origin := s.base.Scheme + "://" + s.base.Host
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", strings.TrimRight(s.base.String(), "/")+"/")
	req.Header.Set("Accept", "text/plain")
	req.Header.Set("User-Agent", "Louro/0.5")
	if method == "POST" {
		req.Header.Set("Content-Type", contentType)
	}
	response, err := s.client.Do(req)
	if err != nil {
		return nil, networkFailure(ctx, err)
	}
	defer response.Body.Close()
	switch {
	case response.StatusCode == 401 || response.StatusCode == 403:
		return nil, failed("authentication-failed")
	case response.StatusCode == 429:
		delay, err := strconv.Atoi(response.Header.Get("Retry-After"))
		if err != nil {
			if at, e := http.ParseTime(response.Header.Get("Retry-After")); e == nil {
				delay = int(time.Until(at).Seconds()) + 1
			} else {
				delay = 60
			}
		}
		if delay < 1 {
			delay = 1
		}
		if delay > 3600 {
			delay = 3600
		}
		return nil, &ProviderError{Code: "download-client-rate-limited", RetryAfter: delay}
	case response.StatusCode >= 300 && response.StatusCode < 400:
		return nil, failed("invalid-response")
	case response.StatusCode != 200:
		return nil, failed("unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if int64(len(data)) > limit {
		return nil, failed("invalid-response")
	}
	if err != nil {
		return nil, networkFailure(ctx, err)
	}
	return data, nil
}
func networkFailure(ctx context.Context, err error) error {
	var timeout net.Error
	if ctx.Err() != nil || errors.As(err, &timeout) && timeout.Timeout() {
		return failed("timeout")
	}
	return failed("unavailable")
}
