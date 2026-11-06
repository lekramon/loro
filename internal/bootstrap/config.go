package bootstrap

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

type Config struct {
	TMDBTokenFile string
	DataDir       string
	Listen        string
	Workers       int
	Container     bool
}

func Parse(args []string, getenv func(string) string) (Config, error) {
	container := false
	if value := Environment(getenv, "CONTAINER"); value != "" {
		var err error
		container, err = strconv.ParseBool(value)
		if err != nil {
			return Config{}, errors.New("container setting must be a boolean")
		}
	}
	dataDefault := "/var/lib/piratarr"
	if runtime.GOOS == "windows" {
		local := getenv("LOCALAPPDATA")
		if local == "" {
			return Config{}, errors.New("LOCALAPPDATA must be set on Windows")
		}
		dataDefault = filepath.Join(local, "Piratarr")
	}
	listenDefault := "127.0.0.1:8080"
	if container {
		dataDefault, listenDefault = "/config", "0.0.0.0:8080"
	}
	envOr := func(name, fallback string) string {
		if v := Environment(getenv, name); v != "" {
			return v
		}
		return fallback
	}
	flags := flag.NewFlagSet("louro", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	tokenFile := flags.String("tmdb-token-file", envOr("TMDB_TOKEN_FILE", ""), "private application metadata credential file")
	data := flags.String("data-dir", envOr("DATA_DIR", dataDefault), "private data directory")
	listen := flags.String("listen", envOr("LISTEN", listenDefault), "HTTP listener")
	workers := flags.String("workers", envOr("WORKERS", "4"), "submission worker count (1-8)")
	if err := flags.Parse(args); err != nil {
		return Config{}, errors.New("invalid command-line options")
	}
	if flags.NArg() != 0 {
		return Config{}, errors.New("unexpected positional arguments")
	}
	count, err := strconv.Atoi(*workers)
	if err != nil || count < 1 || count > 8 {
		return Config{}, errors.New("workers must be between 1 and 8")
	}
	if !filepath.IsAbs(*data) {
		return Config{}, errors.New("data directory must be absolute")
	}
	host, port, err := net.SplitHostPort(*listen)
	if err != nil {
		return Config{}, errors.New("listen must contain an IP address and port")
	}
	ip := net.ParseIP(host)
	number, err := strconv.Atoi(port)
	if ip == nil || err != nil || number < 1 || number > 65535 {
		return Config{}, errors.New("invalid listen address or port")
	}
	if !container && !ip.IsLoopback() {
		return Config{}, errors.New("native listener must use a loopback IP address")
	}
	if *tokenFile != "" && !filepath.IsAbs(*tokenFile) {
		return Config{}, errors.New("TMDB credential file must be absolute")
	}
	return Config{TMDBTokenFile: *tokenFile, DataDir: filepath.Clean(*data), Listen: *listen, Workers: count, Container: container}, nil
}

// PrepareDataDir creates only the private application directory, not media roots.
func PrepareDataDir(path string) (string, error) {
	if err := os.MkdirAll(path, 0700); err != nil {
		return "", fmt.Errorf("create data directory: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve data directory: %w", err)
	}
	if err := secureDirectory(canonical); err != nil {
		return "", fmt.Errorf("secure data directory: %w", err)
	}
	return canonical, nil
}

// Environment keeps Louro configuration compatible with existing deployments.
func Environment(getenv func(string) string, key string) string {
	if value := getenv("LOURO_" + key); value != "" {
		return value
	}
	return getenv("PIRATARR_" + key)
}
