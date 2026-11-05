package bootstrap

import (
	"path/filepath"
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
