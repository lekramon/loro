package sources

import (
	"context"
	"errors"
	"github.com/lekramon/piratarr/internal/communications"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	http       *http.Client
	searchHTTP *http.Client
	mu         sync.Mutex
	cooldown   time.Time
	revision   int
}

const SearchTimeout = 70 * time.Second

func NewClient() *Client {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext, TLSHandshakeTimeout: 2 * time.Second, MaxIdleConns: 4, MaxIdleConnsPerHost: 2, IdleConnTimeout: 30 * time.Second}
	noRedirect := func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{http: &http.Client{Transport: communications.Transport{Base: transport, Integration: "source"}, Timeout: 2 * time.Second, CheckRedirect: noRedirect}, searchHTTP: &http.Client{Transport: communications.Transport{Base: transport, Integration: "source"}, Timeout: 65 * time.Second, CheckRedirect: noRedirect}}
}
func (c *Client) Capabilities(ctx context.Context, config Config) (Capabilities, error) {
	data, err := c.request(ctx, config, url.Values{"t": {"caps"}})
	if err != nil {
		return Capabilities{}, err
	}
	return parseCapabilities(data, config.APIKey)
}

func (c *Client) request(ctx context.Context, config Config, query url.Values) ([]byte, error) {
	search := query.Get("t") != "caps"
	client := c.http
	if search {
		client = c.searchHTTP
	}
	var last error
	for attempt := 0; attempt < 4; attempt++ {
		c.mu.Lock()
		remaining := time.Until(c.cooldown)
		same := c.revision == config.Revision
		c.mu.Unlock()
		if same && remaining > 0 {
			return nil, &ProviderError{Code: "source-rate-limited", RetryAfter: int(remaining.Seconds()) + 1}
		}
		endpoint, err := url.Parse(config.URL)
		if err != nil {
			return nil, failed("source-invalid-response")
		}
		if config.APIKey != "" {
			query.Set("apikey", config.APIKey)
		}
		endpoint.RawQuery = query.Encode()
		req, err := http.NewRequestWithContext(ctx, "GET", endpoint.String(), nil)
		if err != nil {
			return nil, failed("source-invalid-response")
		}
		req.Header.Set("Accept", "application/xml, text/xml")
		req.Header.Set("User-Agent", "Louro/0.4")
		response, err := client.Do(req)
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
					return nil, failed("source-invalid-response")
				}
				if readErr == nil {
					if challenge(response.Header, bytes) {
						return nil, failed("source-challenge-required")
					}
					return bytes, nil
				}
				last = failed("source-unavailable")
				var network net.Error
				if errors.As(readErr, &network) && network.Timeout() {
					last = failed("source-timeout")
				}
			} else {
				body, _ := io.ReadAll(io.LimitReader(response.Body, 65536))
				response.Body.Close()
				if challenge(response.Header, body) {
					return nil, failed("source-challenge-required")
				}
				switch {
				case status == 401 || status == 403:
					return nil, failed("source-authentication-failed")
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
					return nil, &ProviderError{Code: "source-rate-limited", RetryAfter: delay}
				case status >= 500:
					last = failed("source-unavailable")
				case status >= 300 && status < 400:
					return nil, failed("source-invalid-response")
				default:
					return nil, failed("source-unavailable")
				}
			}
		}
		// A slow query may still be executing upstream. Do not multiply that work.
		var provider *ProviderError
		if search && errors.As(last, &provider) && provider.Code == "source-timeout" {
			return nil, last
		}
		if ctx.Err() != nil {
			return nil, failed("source-timeout")
		}
		if attempt < 3 {
			timer := time.NewTimer(time.Duration(100*(1<<attempt)) * time.Millisecond)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return nil, failed("source-timeout")
			}
		}
	}
	return nil, last
}

func challenge(headers http.Header, body []byte) bool {
	if strings.EqualFold(headers.Get("Cf-Mitigated"), "challenge") {
		return true
	}
	text := strings.ToLower(string(body))
	return strings.Contains(text, "<html") && (strings.Contains(text, "/cdn-cgi/challenge-platform/") || strings.Contains(text, "cf-chl-") || strings.Contains(text, "just a moment") && strings.Contains(text, "cloudflare"))
}
