// Package healthcheck probes local readiness without opening application storage.
package healthcheck

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

// Run performs a bounded loopback-only readiness check. It never takes the
// data-directory lock and ignores proxy configuration and redirects.
func Run(ctx context.Context, args []string, getenv func(string) string) error {
	listen := getenv("PIRATARR_LISTEN")
	if listen == "" {
		listen = "127.0.0.1:8080"
	}
	flags := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	target := flags.String("listen", listen, "local application listener")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return errors.New("invalid healthcheck options")
	}
	host, port, err := net.SplitHostPort(*target)
	number, e := strconv.Atoi(port)
	ip := net.ParseIP(host)
	if err != nil || e != nil || number < 1 || number > 65535 || ip == nil || (!ip.IsLoopback() && !ip.IsUnspecified()) {
		return errors.New("healthcheck requires a local IP listener and valid port")
	}
	if ip.IsUnspecified() {
		if ip.To4() != nil {
			host = "127.0.0.1"
		} else {
			host = "::1"
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(host, port)+"/api/v1/health/ready", nil)
	if err != nil {
		return errors.New("healthcheck request could not be created")
	}
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return errors.New("local readiness check failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("local service is not ready")
	}
	var status struct {
		Status string `json:"status"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1024))
	if decoder.Decode(&status) != nil || status.Status != "ready" {
		return errors.New("invalid readiness response")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("invalid readiness response")
	}
	return nil
}
