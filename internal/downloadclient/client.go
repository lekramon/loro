package downloadclient

import (
	"context"
	"errors"
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

type session struct {
	client *http.Client
	base   *url.URL
}

func testConnection(ctx context.Context, config Config) (string, string, error) {
	jar, _ := cookiejar.New(nil)
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext, TLSHandshakeTimeout: 2 * time.Second, ResponseHeaderTimeout: 2 * time.Second, MaxIdleConns: 1, MaxIdleConnsPerHost: 1}
	defer transport.CloseIdleConnections()
	base, err := url.Parse(config.URL)
	if err != nil {
		return "", "", failed("invalid-response")
	}
	s := session{client: &http.Client{Transport: transport, Jar: jar, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, base: base}
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
			cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, _ = s.request(cleanup, "POST", "auth/logout", "")
		}()
	}
	if err != nil {
		return "", "", err
	}
	if login == "Fails." {
		return "", "", failed("authentication-failed")
	}
	if login != "Ok." || !hasSID {
		return "", "", failed("invalid-response")
	}
	app, err := s.request(ctx, "GET", "app/version", "")
	if err != nil {
		return "", "", err
	}
	api, err := s.request(ctx, "GET", "app/webapiVersion", "")
	if err != nil {
		return "", "", err
	}
	appMatch := appPattern.FindStringSubmatch(app)
	apiMatch := apiPattern.FindStringSubmatch(api)
	if appMatch == nil || apiMatch == nil {
		return "", "", failed("invalid-response")
	}
	if appMatch[1] != "5" || apiMatch[1] != "2" {
		return "", "", failed("unsupported-version")
	}
	return app, api, nil
}

var appPattern = regexp.MustCompile(`^v?([0-9]{1,6})\.[0-9]{1,6}\.[0-9]{1,6}$`)
var apiPattern = regexp.MustCompile(`^([0-9]{1,6})\.[0-9]{1,6}(?:\.[0-9]{1,6})?$`)

func (s session) endpoint(route string) *url.URL {
	result := *s.base
	result.RawPath = strings.TrimRight(result.EscapedPath(), "/") + "/api/v2/" + route
	result.Path, _ = url.PathUnescape(result.RawPath) // Parsed base and fixed API route are already valid.
	return &result
}
func (s session) request(ctx context.Context, method, route, body string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, method, s.endpoint(route).String(), strings.NewReader(body))
	if err != nil {
		return "", failed("invalid-response")
	}
	origin := s.base.Scheme + "://" + s.base.Host
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", strings.TrimRight(s.base.String(), "/")+"/")
	req.Header.Set("Accept", "text/plain")
	req.Header.Set("User-Agent", "Piratarr/0.5")
	if method == "POST" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	response, err := s.client.Do(req)
	if err != nil {
		return "", networkFailure(ctx, err)
	}
	defer response.Body.Close()
	switch {
	case response.StatusCode == 401 || response.StatusCode == 403:
		return "", failed("authentication-failed")
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
		return "", &ProviderError{Code: "download-client-rate-limited", RetryAfter: delay}
	case response.StatusCode >= 300 && response.StatusCode < 400:
		return "", failed("invalid-response")
	case response.StatusCode != 200:
		return "", failed("unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if len(data) > 4096 {
		return "", failed("invalid-response")
	}
	if err != nil {
		return "", networkFailure(ctx, err)
	}
	return strings.TrimSpace(string(data)), nil
}
func networkFailure(ctx context.Context, err error) error {
	var timeout net.Error
	if ctx.Err() != nil || errors.As(err, &timeout) && timeout.Timeout() {
		return failed("timeout")
	}
	return failed("unavailable")
}
