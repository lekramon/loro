package library

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func init() {
	mode := os.Getenv("PIRATARR_PROBE_HELPER")
	if mode == "" {
		return
	}
	args := strings.Join(os.Args, " ")
	if !strings.Contains(args, "-protocol_whitelist pipe") || !strings.HasSuffix(args, "-i pipe:0") {
		os.Exit(7)
	}
	bytes, _ := io.ReadAll(os.Stdin)
	if string(bytes) != "video fixture" {
		os.Exit(8)
	}
	fmt.Fprint(os.Stderr, "private provider payload must not escape")
	switch mode {
	case "video":
		fmt.Print(`{"streams":[{"codec_type":"video"}]}`)
	case "audio":
		fmt.Print(`{"streams":[{"codec_type":"audio"}]}`)
	case "bad":
		fmt.Print(`invalid`)
	case "large":
		fmt.Print(strings.Repeat("x", 70000))
	case "timeout":
		time.Sleep(time.Minute)
	}
	os.Exit(0)
}
func TestProbeBoundedProcessAndStreams(t *testing.T) {
	for _, mode := range []string{"video", "audio", "bad", "large", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("PIRATARR_PROBE_HELPER", mode)
			file, err := os.CreateTemp(t.TempDir(), "video")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			file.WriteString("video fixture")
			file.Seek(0, 0)
			ctx := context.Background()
			if mode == "timeout" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 50*time.Millisecond)
				defer cancel()
			}
			err = NewProber(os.Args[0])(ctx, file)
			if mode == "video" {
				if err != nil {
					t.Fatal(err)
				}
			} else if mode == "timeout" {
				if !errors.Is(err, ErrProbeTimeout) {
					t.Fatal(err)
				}
			} else if !errors.Is(err, ErrInvalidVideo) {
				t.Fatal(mode, err)
			}
		})
	}
	file, err := os.CreateTemp(t.TempDir(), "missing")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err = NewProber("piratarr-ffprobe-does-not-exist")(context.Background(), file); !errors.Is(err, ErrProbeUnavailable) {
		t.Fatal(err)
	}
}
func TestRealFFprobe(t *testing.T) {
	path := os.Getenv("PIRATARR_REAL_FFPROBE_FIXTURE")
	if path == "" {
		t.Skip("real fixture supplied by runtime-container check")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err = NewProber("ffprobe")(context.Background(), file); err != nil {
		t.Fatal(err)
	}
}
func TestFilenameSuggestions(t *testing.T) {
	for _, input := range []struct {
		path, title string
		year        int
	}{{"Test.Movie.2024.1080p.mkv", "Test Movie", 2024}, {"The Film (2023)/movie.mkv", "The Film", 2023}, {"unknown.mkv", "unknown", 0}} {
		entry := Entry{RelativePath: input.path}
		Suggest(&entry)
		if entry.SuggestedTitle != input.title || (input.year != 0 && (entry.SuggestedYear == nil || *entry.SuggestedYear != input.year)) {
			t.Fatal(entry)
		}
	}
}
