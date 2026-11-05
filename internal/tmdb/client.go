// Package tmdb implements the fixed-origin metadata provider adapter.
package tmdb

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/lekramon/piratarr/internal/catalog"
)

// Populated only by the maintainer's BuildKit secret during release compilation.
//go:embed release-credential.txt
var releaseCredential string

type Config struct {
	LocalTokenFile string
	TokenFile      string
	Token          func() string
	APIBase        string
	ImageBase      string
}
type Client struct {
	config   Config
	http     *http.Client
	gate     chan struct{}
	mu       sync.Mutex
	cooldown time.Time
}

func New(config Config) *Client {
	if config.APIBase == "" {
		config.APIBase = "https://api.themoviedb.org/3"
	}
	if config.ImageBase == "" {
		config.ImageBase = "https://image.tmdb.org/t/p"
	}
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext, TLSHandshakeTimeout: 2 * time.Second, ResponseHeaderTimeout: 2 * time.Second, MaxIdleConns: 8, MaxIdleConnsPerHost: 4, IdleConnTimeout: 30 * time.Second}
	return &Client{config: config, http: &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, gate: make(chan struct{}, 4)}
}
func (c *Client) credential() string {
	if c.config.LocalTokenFile != "" {
		if _, err := os.Lstat(c.config.LocalTokenFile); err == nil || !os.IsNotExist(err) {
			return (&Client{config: Config{TokenFile: c.config.LocalTokenFile}}).credential()
		}
	}
	value := releaseCredential
	if c.config.Token != nil {
		value = c.config.Token()
	} else if c.config.TokenFile != "" {
		info, err := os.Stat(c.config.TokenFile)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 8192 {
			return ""
		}
		file, err := os.Open(c.config.TokenFile)
		if err != nil {
			return ""
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, 8193))
		if err != nil || len(data) > 8192 {
			return ""
		}
		value = string(data)
	}
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 8192 {
		return ""
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-') {
			return ""
		}
	}
	return value
}
func (c *Client) Configured() bool { return c.credential() != "" }
func failure(code string) error    { return &catalog.ProviderError{Code: code} }
func (c *Client) request(ctx context.Context, target string, authenticated bool, limit int64) ([]byte, error) {
	var last error
	for attempt := 0; attempt < 4; attempt++ {
		c.mu.Lock()
		remaining := time.Until(c.cooldown)
		c.mu.Unlock()
		if remaining > 0 {
			return nil, &catalog.ProviderError{Code: "metadata-rate-limited", RetryAfter: int(remaining.Seconds()) + 1}
		}
		token := ""
		if authenticated {
			token = c.credential()
			if token == "" {
				return nil, failure("metadata-unconfigured")
			}
		}
		select {
		case c.gate <- struct{}{}:
		case <-ctx.Done():
			return nil, failure("metadata-timeout")
		}
		c.mu.Lock()
		remaining = time.Until(c.cooldown)
		c.mu.Unlock()
		if remaining > 0 {
			<-c.gate
			return nil, &catalog.ProviderError{Code: "metadata-rate-limited", RetryAfter: int(remaining.Seconds()) + 1}
		}
		attemptCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, target, nil)
		if err != nil {
			cancel()
			<-c.gate
			return nil, failure("metadata-invalid-response")
		}
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if authenticated {
			req.Header.Set("Accept", "application/json")
		} else {
			req.Header.Set("Accept", "image/jpeg, image/png")
		}
		req.Header.Set("User-Agent", "Louro/0.2")
		response, err := c.http.Do(req)
		if err != nil {
			last = failure("metadata-unavailable")
			var network net.Error
			if errors.As(err, &network) && network.Timeout() {
				last = failure("metadata-timeout")
			}
			cancel()
			<-c.gate
		} else {
			status := response.StatusCode
			if status == 200 {
				data, readErr := io.ReadAll(io.LimitReader(response.Body, limit+1))
				response.Body.Close()
				cancel()
				<-c.gate
				if readErr != nil {
					var network net.Error
					if ctx.Err() != nil || errors.Is(readErr, context.DeadlineExceeded) || (errors.As(readErr, &network) && network.Timeout()) {
						return nil, failure("metadata-timeout")
					}
					return nil, failure("metadata-unavailable")
				}
				if int64(len(data)) > limit {
					return nil, failure("metadata-invalid-response")
				}
				return data, nil
			}
			retryHeader := response.Header.Get("Retry-After")
			response.Body.Close()
			cancel()
			<-c.gate
			switch {
			case status == 401 || status == 403:
				return nil, failure("metadata-authentication-failed")
			case status == 404:
				return nil, failure("metadata-not-found")
			case status == 429:
				seconds, parseErr := strconv.Atoi(retryHeader)
				if parseErr != nil {
					if at, e := http.ParseTime(retryHeader); e == nil {
						seconds = int(time.Until(at).Seconds()) + 1
					} else {
						seconds = 60
					}
				}
				if seconds < 1 {
					seconds = 1
				}
				if seconds > 3600 {
					seconds = 3600
				}
				c.mu.Lock()
				c.cooldown = time.Now().Add(time.Duration(seconds) * time.Second)
				c.mu.Unlock()
				return nil, &catalog.ProviderError{Code: "metadata-rate-limited", RetryAfter: seconds}
			case status >= 500:
				last = failure("metadata-unavailable")
			default:
				return nil, failure("metadata-invalid-response")
			}
		}
		if ctx.Err() != nil {
			return nil, failure("metadata-timeout")
		}
		if attempt < 3 {
			timer := time.NewTimer(time.Duration(100*(1<<attempt)) * time.Millisecond)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return nil, failure("metadata-timeout")
			}
		}
	}
	return nil, last
}

type movieDTO struct {
	ID               int64  `json:"id"`
	Title            string `json:"title"`
	OriginalTitle    string `json:"original_title"`
	OriginalLanguage string `json:"original_language"`
	Overview         string `json:"overview"`
	ReleaseDate      string `json:"release_date"`
	Runtime          *int   `json:"runtime"`
	Genres           []struct {
		Name string `json:"name"`
	} `json:"genres"`
	PosterPath   string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
}

var imagePath = regexp.MustCompile(`^/[A-Za-z0-9]+\.(jpg|png)$`)

func normalize(dto movieDTO, language string) (catalog.Metadata, error) {
	title := strings.TrimSpace(dto.Title)
	if title == "" {
		title = strings.TrimSpace(dto.OriginalTitle)
	}
	if dto.ID < 1 || dto.ID > 2147483647 || title == "" || utf8.RuneCountInString(title) > 500 {
		return catalog.Metadata{}, failure("metadata-invalid-response")
	}
	result := catalog.Metadata{TMDBID: dto.ID, Title: title, OriginalTitle: dto.OriginalTitle, OriginalLanguage: dto.OriginalLanguage, Overview: dto.Overview, RuntimeMinutes: dto.Runtime, Genres: []string{}, MetadataLanguage: language}
	if result.RuntimeMinutes != nil && *result.RuntimeMinutes <= 0 {
		result.RuntimeMinutes = nil
	}
	if date, err := time.Parse("2006-01-02", dto.ReleaseDate); err == nil {
		year := date.Year()
		result.Year = &year
	}
	for _, genre := range dto.Genres {
		if genre.Name != "" {
			result.Genres = append(result.Genres, genre.Name)
		}
	}
	if imagePath.MatchString(dto.PosterPath) {
		result.PosterURL = catalog.ImagePrefix + "w342" + dto.PosterPath
	}
	if imagePath.MatchString(dto.BackdropPath) {
		result.BackdropURL = catalog.ImagePrefix + "w780" + dto.BackdropPath
	}
	return result, nil
}
func (c *Client) Search(ctx context.Context, query string, year, page int, language string) (catalog.SearchResult, error) {
	parameters := url.Values{"query": {query}, "language": {language}, "page": {strconv.Itoa(page)}, "include_adult": {"false"}}
	if year > 0 {
		parameters.Set("year", strconv.Itoa(year))
	}
	data, err := c.request(ctx, strings.TrimRight(c.config.APIBase, "/")+"/search/movie?"+parameters.Encode(), true, 1<<20)
	if err != nil {
		return catalog.SearchResult{}, err
	}
	var dto struct {
		Page         int        `json:"page"`
		TotalPages   int        `json:"total_pages"`
		TotalResults int        `json:"total_results"`
		Results      []movieDTO `json:"results"`
	}
	if !utf8.Valid(data) || json.Unmarshal(data, &dto) != nil || dto.Page != page || dto.Results == nil || len(dto.Results) > 20 || dto.TotalPages < 0 || dto.TotalResults < 0 {
		return catalog.SearchResult{}, failure("metadata-invalid-response")
	}
	result := catalog.SearchResult{Page: dto.Page, TotalPages: dto.TotalPages, TotalResults: dto.TotalResults, Results: []catalog.Metadata{}}
	for _, movie := range dto.Results {
		normalized, err := normalize(movie, language)
		if err != nil {
			return catalog.SearchResult{}, err
		}
		result.Results = append(result.Results, normalized)
	}
	return result, nil
}
func (c *Client) Details(ctx context.Context, id int64, language string) (catalog.Metadata, error) {
	data, err := c.request(ctx, strings.TrimRight(c.config.APIBase, "/")+"/movie/"+strconv.FormatInt(id, 10)+"?language="+url.QueryEscape(language), true, 1<<20)
	if err != nil {
		return catalog.Metadata{}, err
	}
	var dto movieDTO
	if !utf8.Valid(data) || json.Unmarshal(data, &dto) != nil || dto.ID != id {
		return catalog.Metadata{}, failure("metadata-invalid-response")
	}
	return normalize(dto, language)
}
func (c *Client) Image(ctx context.Context, size, filename string) ([]byte, error) {
	if (size != "w342" && size != "w780") || !imagePath.MatchString("/"+filename) {
		return nil, failure("metadata-not-found")
	}
	return c.request(ctx, strings.TrimRight(c.config.ImageBase, "/")+"/"+size+"/"+filename, false, 8<<20)
}
