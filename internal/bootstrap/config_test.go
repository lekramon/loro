package bootstrap

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestBootstrapPrecedenceAndValidation(t *testing.T) {
	root := t.TempDir()
	environment := map[string]string{"LOCALAPPDATA": root, "PIRATARR_DATA_DIR": filepath.Join(root, "environment"), "PIRATARR_LISTEN": "127.0.0.1:8181", "PIRATARR_WORKERS": "2"}
	getenv := func(key string) string { return environment[key] }
	config, err := Parse([]string{"--data-dir", filepath.Join(root, "flag"), "--workers", "6"}, getenv)
	if err != nil {
		t.Fatal(err)
	}
	if config.DataDir != filepath.Join(root, "flag") || config.Workers != 6 || config.Listen != "127.0.0.1:8181" {
		t.Fatalf("incorrect precedence: %+v", config)
	}
	for _, args := range [][]string{{"--workers", "0"}, {"--workers", "9"}, {"--workers", "wrong"}, {"--listen", "0.0.0.0:8080"}, {"--listen", "127.0.0.1:0"}, {"--data-dir", "relative"}, {"--unknown"}, {"extra"}} {
		if _, err := Parse(args, getenv); err == nil {
			t.Errorf("expected invalid options: %v", args)
		}
	}
	environment["PIRATARR_WORKERS"] = "broken"
	if _, err := Parse([]string{"--workers", "3"}, getenv); err != nil {
		t.Fatalf("valid flag must override invalid lower-precedence environment: %v", err)
	}
	environment["PIRATARR_CONTAINER"] = "true"
	environment["PIRATARR_WORKERS"] = "4"
	if _, err := Parse([]string{"--listen", "0.0.0.0:8080"}, getenv); err != nil {
		t.Fatal(err)
	}
}

func TestLouroEnvironmentPreservesLegacyDataIdentity(t *testing.T) {
	root := t.TempDir()
	env := map[string]string{"LOCALAPPDATA": root, "PIRATARR_DATA_DIR": filepath.Join(root, "legacy"), "PIRATARR_LISTEN": "127.0.0.1:8081", "PIRATARR_WORKERS": "2"}
	getenv := func(key string) string { return env[key] }
	old, err := Parse(nil, getenv)
	if err != nil {
		t.Fatal(err)
	}
	env["LOURO_LISTEN"] = "127.0.0.1:8082"
	env["LOURO_WORKERS"] = "3"
	current, err := Parse(nil, getenv)
	if err != nil || current.DataDir != old.DataDir || current.Listen != "127.0.0.1:8082" || current.Workers != 3 {
		t.Fatal(current, err)
	}
	env["LOURO_DATA_DIR"] = filepath.Join(root, "chosen")
	env["LOURO_TMDB_TOKEN_FILE"] = filepath.Join(root, "token")
	current, err = Parse([]string{"--data-dir", old.DataDir, "--listen", old.Listen}, getenv)
	if err != nil || current.DataDir != old.DataDir || current.Listen != old.Listen || current.TMDBTokenFile != env["LOURO_TMDB_TOKEN_FILE"] {
		t.Fatal(current, err)
	}
	env["LOURO_WORKERS"] = "invalid"
	if _, err = Parse(nil, getenv); err == nil {
		t.Fatal("invalid preferred value fell through to legacy")
	}
	env = map[string]string{"LOCALAPPDATA": root}
	current, err = Parse(nil, getenv)
	if err != nil {
		t.Fatal(err)
	}
	expected := "/var/lib/piratarr"
	if runtime.GOOS == "windows" {
		expected = filepath.Join(root, "Piratarr")
	}
	if current.DataDir != expected {
		t.Fatal("default moved existing installations", current.DataDir)
	}
	env["LOURO_CONTAINER"] = "true"
	containerArgs := []string{}
	containerRoot := "/config"
	if runtime.GOOS == "windows" {
		containerRoot = filepath.Join(root, "container")
		containerArgs = []string{"--data-dir", containerRoot}
	}
	current, err = Parse(containerArgs, getenv)
	if err != nil || current.DataDir != containerRoot || current.Listen != "0.0.0.0:8080" {
		t.Fatal(current, err)
	}
}
