package library

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"time"
)

var (
	ErrNotFound         = errors.New("file or movie not found")
	ErrConflict         = errors.New("file review changed")
	ErrInvalidVideo     = errors.New("a nonzero valid video is required")
	ErrProbeUnavailable = errors.New("ffprobe unavailable")
	ErrProbeTimeout     = errors.New("ffprobe timed out")
)

type Prober func(context.Context, *os.File) error
type limitedOutput struct{ bytes.Buffer }

func (b *limitedOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 65536 {
		return 0, ErrInvalidVideo
	}
	return b.Buffer.Write(p)
}
func NewProber(executable string) Prober {
	return func(ctx context.Context, file *os.File) error {
		ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, executable, "-v", "error", "-protocol_whitelist", "pipe", "-show_entries", "stream=codec_type", "-of", "json", "-i", "pipe:0")
		output := &limitedOutput{}
		cmd.Stdin = file
		cmd.Stdout = output
		cmd.Stderr = io.Discard
		err := cmd.Run()
		if ctx.Err() != nil {
			return ErrProbeTimeout
		}
		if err != nil {
			var exited *exec.ExitError
			if errors.As(err, &exited) || errors.Is(err, ErrInvalidVideo) {
				return ErrInvalidVideo
			}
			return ErrProbeUnavailable
		}
		var result struct {
			Streams []struct {
				Type string `json:"codec_type"`
			} `json:"streams"`
		}
		if json.Unmarshal(output.Bytes(), &result) != nil {
			return ErrInvalidVideo
		}
		for _, stream := range result.Streams {
			if stream.Type == "video" {
				return nil
			}
		}
		return ErrInvalidVideo
	}
}
