package sources

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type ProviderError struct {
	Code       string
	RetryAfter int
}

func (e *ProviderError) Error() string { return e.Code }
func failed(code string) error         { return &ProviderError{Code: code} }

type Client struct {
	http     *http.Client
	mu       sync.Mutex
	cooldown time.Time
	revision int
}

func NewClient() *Client {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext, TLSHandshakeTimeout: 2 * time.Second, ResponseHeaderTimeout: 2 * time.Second, MaxIdleConns: 4, MaxIdleConnsPerHost: 2, IdleConnTimeout: 30 * time.Second}
	return &Client{http: &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *Client) Capabilities(ctx context.Context, config Config) (Capabilities, error) {
	var last error
	for attempt := 0; attempt < 4; attempt++ {
		c.mu.Lock()
		remaining := time.Until(c.cooldown)
		same := c.revision == config.Revision
		c.mu.Unlock()
		if same && remaining > 0 {
			return Capabilities{}, &ProviderError{Code: "source-rate-limited", RetryAfter: int(remaining.Seconds()) + 1}
		}
		endpoint, err := url.Parse(config.URL)
		if err != nil {
			return Capabilities{}, failed("source-invalid-response")
		}
		query := url.Values{"t": {"caps"}}
		if config.APIKey != "" {
			query.Set("apikey", config.APIKey)
		}
		endpoint.RawQuery = query.Encode()
		req, err := http.NewRequestWithContext(ctx, "GET", endpoint.String(), nil)
		if err != nil {
			return Capabilities{}, failed("source-invalid-response")
		}
		req.Header.Set("Accept", "application/xml, text/xml")
		req.Header.Set("User-Agent", "Piratarr/0.4")
		response, err := c.http.Do(req)
		if err != nil {
			last = failed("source-unavailable")
			var network net.Error
			if errors.As(err, &network) && network.Timeout() {
				last = failed("source-timeout")
			}
		} else {
			status := response.StatusCode
			if status == 200 {
				bytes, readErr := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
				response.Body.Close()
				if len(bytes) > 2<<20 {
					return Capabilities{}, failed("source-invalid-response")
				}
				if readErr == nil {
					return parseCapabilities(bytes, config.APIKey)
				}
				last = failed("source-unavailable")
				var network net.Error
				if errors.As(readErr, &network) && network.Timeout() {
					last = failed("source-timeout")
				}
			} else {
				response.Body.Close()
				switch {
				case status == 401 || status == 403:
					return Capabilities{}, failed("source-authentication-failed")
				case status == 429:
					delay, err := strconv.Atoi(response.Header.Get("Retry-After"))
					if err != nil {
						if deadline, e := http.ParseTime(response.Header.Get("Retry-After")); e == nil {
							delay = int(time.Until(deadline).Seconds()) + 1
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
					c.mu.Lock()
					c.cooldown = time.Now().Add(time.Duration(delay) * time.Second)
					c.revision = config.Revision
					c.mu.Unlock()
					return Capabilities{}, &ProviderError{Code: "source-rate-limited", RetryAfter: delay}
				case status >= 500:
					last = failed("source-unavailable")
				case status >= 300 && status < 400:
					return Capabilities{}, failed("source-invalid-response")
				default:
					return Capabilities{}, failed("source-unavailable")
				}
			}
		}
		if ctx.Err() != nil {
			return Capabilities{}, failed("source-timeout")
		}
		if attempt < 3 {
			timer := time.NewTimer(time.Duration(100*(1<<attempt)) * time.Millisecond)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return Capabilities{}, failed("source-timeout")
			}
		}
	}
	return Capabilities{}, last
}
