package acquisitions

import (
	"context"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func normalizeHash(value string) (string, bool) {
	if len(value) == 40 {
		data, err := hex.DecodeString(value)
		if err == nil && len(data) == 20 {
			return hex.EncodeToString(data), true
		}
	}
	if len(value) == 32 {
		data, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(value))
		if err == nil && len(data) == 20 {
			return hex.EncodeToString(data), true
		}
	}
	return "", false
}
func magnetIdentity(reference string) (string, string) {
	u, err := url.Parse(reference)
	if err != nil || !strings.EqualFold(u.Scheme, "magnet") || u.Host != "" || u.Path != "" || u.Opaque != "" || u.Fragment != "" {
		return "", "invalid-magnet"
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", "invalid-magnet"
	}
	hash := ""
	for _, xt := range values["xt"] {
		if !strings.HasPrefix(strings.ToLower(xt), "urn:btih:") {
			return "", "unsupported-identity"
		}
		value, ok := normalizeHash(xt[len("urn:btih:"):])
		if !ok {
			return "", "invalid-magnet"
		}
		if hash != "" && value != hash {
			return "", "identity-mismatch"
		}
		hash = value
	}
	if hash == "" {
		return "", "unsupported-identity"
	}
	return hash, ""
}
func origin(u *url.URL) (string, bool) {
	if u == nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false
	}
	port := 80
	if u.Scheme == "https" {
		port = 443
	}
	if u.Port() != "" {
		n, err := strconv.Atoi(u.Port())
		if err != nil || n < 1 || n > 65535 {
			return "", false
		}
		port = n
	}
	return u.Scheme + "://" + strings.ToLower(u.Hostname()) + ":" + strconv.Itoa(port), true
}
func resolve(ctx context.Context, record Record) Preparation {
	if record.ReferenceIssue != "" {
		return Preparation{ReviewCode: record.ReferenceIssue}
	}
	if len(record.Reference) == 0 {
		return Preparation{ReviewCode: "missing-reference"}
	}
	if strings.Contains(record.Reference, "#") || len(record.Reference) > 8192 {
		return Preparation{ReviewCode: "unsupported-reference"}
	}
	u, err := url.Parse(record.Reference)
	if err != nil {
		return Preparation{ReviewCode: "unsupported-reference"}
	}
	result := Preparation{}
	if strings.EqualFold(u.Scheme, "magnet") {
		result.Hash, result.ReviewCode = magnetIdentity(record.Reference)
		result.Kind = "magnet"
	} else {
		base, err := url.Parse(record.SourceURL)
		if err != nil {
			return Preparation{ReviewCode: "unsupported-reference"}
		}
		if u.User != nil || u.Fragment != "" {
			return Preparation{ReviewCode: "unsupported-reference"}
		}
		u = base.ResolveReference(u)
		from, ok := origin(base)
		to, valid := origin(u)
		if !ok || !valid || from != to {
			return Preparation{ReviewCode: "unsupported-reference"}
		}
		data, code := fetchMetainfo(ctx, u.String())
		if code != "" {
			return Preparation{ReviewCode: code}
		}
		result.Hash, result.ReviewCode = torrentIdentity(data)
		result.Kind = "torrent"
		result.Payload = data
	}
	if result.ReviewCode != "" {
		return Preparation{ReviewCode: result.ReviewCode}
	}
	if record.HashClaim != "" {
		claim, ok := normalizeHash(record.HashClaim)
		if !ok || claim != result.Hash {
			return Preparation{ReviewCode: "identity-mismatch"}
		}
	}
	return result
}
func fetchMetainfo(ctx context.Context, address string) ([]byte, string) {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, DialContext: (&net.Dialer{Timeout: 4 * time.Second}).DialContext, TLSHandshakeTimeout: 4 * time.Second, ResponseHeaderTimeout: 4 * time.Second}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 4 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(ctx, "GET", address, nil)
	if err != nil {
		return nil, "unsupported-reference"
	}
	req.Header.Set("Accept", "application/x-bittorrent")
	req.Header.Set("User-Agent", "Louro/0.6")
	response, err := client.Do(req)
	if err != nil {
		return nil, transportCode(ctx, err)
	}
	defer response.Body.Close()
	switch {
	case response.StatusCode == 401 || response.StatusCode == 403:
		return nil, "source-authentication-failed"
	case response.StatusCode == 429:
		return nil, "source-rate-limited"
	case response.StatusCode >= 300 && response.StatusCode < 400:
		return nil, "unsupported-reference"
	case response.StatusCode != 200:
		return nil, "source-unavailable"
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	if len(data) > 4<<20 {
		return nil, "torrent-too-large"
	}
	if err != nil {
		return nil, transportCode(ctx, err)
	}
	return data, ""
}
func transportCode(ctx context.Context, err error) string {
	var timeout net.Error
	if ctx.Err() != nil || errors.As(err, &timeout) && timeout.Timeout() {
		return "source-timeout"
	}
	return "source-unavailable"
}
