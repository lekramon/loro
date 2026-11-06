package downloads

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/lekramon/piratarr/internal/downloadclient"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const maxInteger int64 = 9007199254740991

type transfer struct {
	Hash        *string  `json:"hash"`
	Category    *string  `json:"category"`
	Tags        *string  `json:"tags"`
	State       *string  `json:"state"`
	Progress    *float64 `json:"progress"`
	Size        *int64   `json:"size"`
	Left        *int64   `json:"amount_left"`
	Speed       *int64   `json:"dlspeed"`
	Completion  *int64   `json:"completion_on"`
	SavePath    *string  `json:"save_path"`
	ContentPath *string  `json:"content_path"`
}

func invalid() error { return &downloadclient.ProviderError{Code: "download-client-invalid-response"} }
func info(ctx context.Context, s *downloadclient.Session, j Job) (transfer, error) {
	data, err := s.Exchange(ctx, "GET", "torrents/info?"+url.Values{"hashes": {j.Hash}}.Encode(), "", nil, 65536)
	if err != nil {
		return transfer{}, err
	}
	var rows []transfer
	if !utf8.Valid(data) || json.Unmarshal(data, &rows) != nil || bytes.Equal(bytes.TrimSpace(data), []byte("null")) || len(rows) > 1 {
		return transfer{}, invalid()
	}
	if len(rows) == 0 {
		return transfer{}, Error("download-torrent-missing")
	}
	r := rows[0]
	if r.Hash == nil || r.Category == nil || r.Tags == nil || len(*r.Hash) != 40 || !strings.EqualFold(*r.Hash, j.Hash) {
		return r, invalid()
	}
	owned := false
	for _, tag := range strings.Split(*r.Tags, ",") {
		tag = strings.TrimSpace(tag)
		if strings.HasPrefix(tag, "louro-acquisition-") && tag != "louro-acquisition-"+j.ID {
			return r, Error("download-ownership-lost")
		}
		if tag == "louro-acquisition-"+j.ID {
			owned = true
		}
	}
	if !owned || *r.Category != "louro" {
		return r, Error("download-ownership-lost")
	}
	if r.State == nil || r.Progress == nil || *r.Progress < 0 || *r.Progress > 1 || !validCount(r.Size) || !validCount(r.Left) || !validCount(r.Speed) || *r.Left > *r.Size {
		return r, invalid()
	}
	return r, nil
}
func validCount(v *int64) bool { return v != nil && *v >= 0 && *v <= maxInteger }
func readTransfer(ctx context.Context, s *downloadclient.Session, j Job, now time.Time) (Result, error) {
	r, err := info(ctx, s, j)
	if err != nil {
		return Result{}, err
	}
	state := "downloading"
	complete := false
	switch *r.State {
	case "downloading", "metaDL", "forcedMetaDL", "stalledDL", "forcedDL", "checkingDL", "checkingUP", "checkingResumeData", "allocating", "moving":
	case "pausedDL", "stoppedDL":
		state = "paused"
	case "queuedDL":
		state = "queued"
	case "error":
		state = "failed"
	case "missingFiles":
		state = "failed"
	case "uploading", "stalledUP", "queuedUP", "forcedUP", "pausedUP", "stoppedUP":
		if *r.Size <= 0 || *r.Progress != 1 || *r.Left != 0 || r.Completion == nil || *r.Completion <= 0 || *r.Completion > now.Unix()+300 {
			return Result{}, Error("download-completion-inconsistent")
		}
		state = "completed"
		complete = true
	default:
		return Result{}, Error("download-state-unsupported")
	}
	result := Result{State: state, Metrics: &Metrics{Progress: r.Progress, SizeBytes: r.Size, RemainingBytes: r.Left, DownloadSpeed: r.Speed, ClientState: r.State}}
	if *r.State == "error" {
		result.Code = "download-transfer-failed"
	}
	if *r.State == "missingFiles" {
		result.Code = "download-files-missing"
	}
	if !complete {
		return result, nil
	}
	handoff, err := readFiles(ctx, s, j, r)
	if err != nil {
		return Result{}, err
	}
	// File selection is an evidence snapshot: fence a completion/ownership/path race with another read.
	fresh, err := info(ctx, s, j)
	if err != nil {
		return Result{}, err
	}
	if !stableCompletion(r, fresh) {
		return Result{}, Error("download-completion-changed")
	}
	stamp := time.Unix(*r.Completion, 0).UTC().Format(time.RFC3339Nano)
	result.CompletedAt = &stamp
	result.Handoff = handoff
	return result, nil
}
func stableCompletion(a, b transfer) bool {
	if b.State == nil {
		return false
	}
	switch *b.State {
	case "uploading", "stalledUP", "queuedUP", "forcedUP", "pausedUP", "stoppedUP":
	default:
		return false
	}
	return b.Progress != nil && *b.Progress == 1 && b.Left != nil && *b.Left == 0 && b.Size != nil && *b.Size == *a.Size && b.Completion != nil && *b.Completion == *a.Completion && b.SavePath != nil && a.SavePath != nil && *b.SavePath == *a.SavePath && b.ContentPath != nil && a.ContentPath != nil && *b.ContentPath == *a.ContentPath
}
