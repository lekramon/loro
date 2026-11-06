package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lekramon/piratarr/internal/downloads"
	"github.com/lekramon/piratarr/internal/settings"
	"github.com/lekramon/piratarr/internal/storage"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type trackingFixture struct {
	*submissionFixture
	infoBody, filesBody        atomic.Value
	mode, infoReads, fileReads atomic.Int32
	entered, release           chan struct{}
	root, sentinel             string
	now                        time.Time
}

func newTrackingFixture(t *testing.T) *trackingFixture {
	f := &trackingFixture{submissionFixture: newSubmissionFixture(t, false), entered: make(chan struct{}, 1), release: make(chan struct{}), now: time.Now().UTC()}
	f.queue(t)
	f.submissionFixture.step(t)
	f.now = time.Now().UTC()
	if f.current(t).Submission.State != "submitted" {
		t.Fatal("submission not confirmed")
	}
	cfg, _ := f.store.GetSettings(context.Background())
	f.root = cfg.Directories.DownloadDirectory
	f.sentinel = filepath.Join(f.root, "preserved.mkv")
	os.WriteFile(f.sentinel, []byte("original local bytes"), 0600)
	f.setInfo(t, "downloading", .25, 100, 75)
	f.setFiles(t, []map[string]any{remoteEvidence("Movie/movie.mkv", 100, 1, 1, 0)})
	f.pollHandler.Store(func(w http.ResponseWriter, r *http.Request) bool {
		route := strings.TrimPrefix(r.URL.Path, "/api/v2/")
		if route != "torrents/info" && route != "torrents/files" {
			return false
		}
		if route == "torrents/info" {
			f.infoReads.Add(1)
			if r.URL.Query().Get("hashes") != *f.ready.InfoHash || r.URL.Query().Has("category") || r.URL.Query().Has("tag") {
				t.Error("poll did not use exact unfiltered identity")
			}
			switch f.mode.Load() {
			case 1:
				fmt.Fprint(w, "[]")
				return true
			case 2:
				fmt.Fprintf(w, `[{"hash":%q,"category":"foreign","tags":""}]`, *f.ready.InfoHash)
				return true
			case 3:
				fmt.Fprint(w, "null")
				return true
			case 4:
				fmt.Fprint(w, strings.Repeat("x", 65537))
				return true
			case 5:
				w.WriteHeader(503)
				return true
			case 6:
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(429)
				return true
			case 7:
				w.Header().Set("Location", f.qb.URL+"/unexpected")
				w.WriteHeader(302)
				return true
			case 8:
				select {
				case f.entered <- struct{}{}:
				default:
				}
				select {
				case <-f.release:
				case <-r.Context().Done():
					return true
				}
			case 9:
				if f.fileReads.Load() > 0 {
					f.setInfo(t, "downloading", .5, 100, 50)
				}
			}
			fmt.Fprint(w, f.infoBody.Load().(string))
			return true
		}
		f.fileReads.Add(1)
		if r.URL.Query().Get("hash") != *f.ready.InfoHash {
			t.Error("file identity changed")
		}
		if f.mode.Load() == 10 {
			fmt.Fprint(w, strings.Repeat("x", (1<<20)+1))
			return true
		}
		fmt.Fprint(w, f.filesBody.Load().(string))
		return true
	})
	return f
}
func remoteEvidence(name string, size int64, progress float64, priority, index int) map[string]any {
	return map[string]any{"name": name, "size": size, "progress": progress, "priority": priority, "index": index}
}
func (f *trackingFixture) setInfo(t *testing.T, state string, progress float64, size, left int64) {
	body, err := json.Marshal([]map[string]any{{"hash": *f.ready.InfoHash, "category": "louro", "tags": "louro-acquisition-" + f.ready.ID, "state": state, "progress": progress, "size": size, "amount_left": left, "dlspeed": int64(12), "completion_on": f.now.Unix() - 1, "save_path": f.root, "content_path": filepath.Join(f.root, "Movie")}})
	if err != nil {
		t.Fatal(err)
	}
	f.infoBody.Store(string(body))
}
func (f *trackingFixture) setFiles(t *testing.T, files []map[string]any) {
	body, err := json.Marshal(files)
	if err != nil {
		t.Fatal(err)
	}
	f.filesBody.Store(string(body))
}
func (f *trackingFixture) task(t *testing.T) downloads.Detail {
	return decodeResponse[downloads.Detail](t, f.request(t, "GET", "/api/v1/downloads/"+f.ready.ID, ""), 200)
}
func (f *trackingFixture) poll(t *testing.T) {
	t.Helper()
	worked, err := (&downloads.Engine{Repo: f.store, Now: func() time.Time { return f.now }}).Step(context.Background())
	if !worked || err != nil {
		t.Fatal("poll", worked, err)
	}
}
func (f *trackingFixture) check(t *testing.T, status int) downloads.Task {
	t.Helper()
	current := f.task(t).Download
	r := f.request(t, "POST", "/api/v1/downloads/"+f.ready.ID+"/check", fmt.Sprintf(`{"revision":%d}`, current.Revision))
	if status != 202 {
		decodeResponse[apiError](t, r, status)
		return current
	}
	value := decodeResponse[downloads.Task](t, r, status)
	if now := time.Now().UTC(); now.After(f.now) {
		f.now = now
	}
	return value
}
func TestDownloadProgressSchedulingCompletionHistoryAndOfflineRead(t *testing.T) {
	f := newTrackingFixture(t)
	initial := f.task(t)
	if initial.Download.State != "pending" || initial.Download.Progress != nil || initial.Download.Handoff != nil || initial.EventTotal != 1 {
		t.Fatal(initial)
	}
	before := f.calls.Load()
	decodeResponse[downloads.List](t, f.request(t, "GET", "/api/v1/downloads", ""), 200)
	if f.calls.Load() != before {
		t.Fatal("read contacted client")
	}
	f.poll(t)
	first := f.task(t)
	if first.Download.State != "downloading" || *first.Download.Progress != .25 || first.Download.LastObservedAt == nil || first.Download.NextCheckAt == nil || first.EventTotal != 2 {
		t.Fatal(first)
	}
	worked, err := (&downloads.Engine{Repo: f.store, Now: func() time.Time { return f.now.Add(14 * time.Second) }}).Step(context.Background())
	if worked || err != nil {
		t.Fatal("poll ran before due", worked, err)
	}
	// A progress-only update is saved without appending an identical-state event.
	f.now = f.now.Add(16 * time.Second)
	f.setInfo(t, "downloading", .5, 100, 50)
	f.poll(t)
	second := f.task(t)
	if second.EventTotal != 2 || *second.Download.Progress != .5 {
		t.Fatal(second)
	}
	f.now = f.now.Add(16 * time.Second)
	f.setInfo(t, "stoppedDL", .5, 100, 50)
	f.poll(t)
	if f.task(t).Download.State != "paused" {
		t.Fatal("pause mapping")
	}
	f.now = f.now.Add(16 * time.Second)
	f.setInfo(t, "stalledUP", 1, 100, 0)
	f.poll(t)
	complete := f.task(t)
	if complete.Download.State != "completed" || complete.Download.PollState != "idle" || complete.Download.Handoff == nil || complete.Download.Handoff.State != "awaiting-validation" || complete.Download.Handoff.File.RelativePath != "Movie/movie.mkv" || f.fileReads.Load() != 1 {
		t.Fatal(complete)
	}
	body, _ := os.ReadFile(f.sentinel)
	if string(body) != "original local bytes" || f.adds.Load() != 1 {
		t.Fatal("poll modified media/client transfer")
	}
	// Check is queue-only and clears import evidence before the worker can observe again.
	before = f.calls.Load()
	queued := f.check(t, 202)
	if queued.Handoff != nil || queued.CompletedAt != nil || queued.State != "pending" || f.calls.Load() != before {
		t.Fatal("check did not invalidate evidence", queued)
	}
	f.mode.Store(5)
	f.now = f.now.Add(time.Second)
	f.poll(t)
	offline := f.task(t)
	if offline.Download.State != "unavailable" || *offline.Download.Progress != 1 || offline.Download.LastObservedAt == nil || offline.Download.Handoff != nil {
		t.Fatal(offline)
	}
	f.store.Close()
	f.store, err = storage.Open(context.Background(), f.data)
	if err != nil {
		t.Fatal(err)
	}
	f.rebind()
	preserved := f.task(t)
	if preserved.Download.State != "unavailable" || preserved.EventTotal != offline.EventTotal {
		t.Fatal("history lost on restart")
	}
	health := f.request(t, "GET", "/api/v1/health/ready", "")
	if health.Code != 200 {
		t.Fatal("client outage broke readiness")
	}
}
func TestDownloadOwnershipOutageCooldownAndMalformedInputs(t *testing.T) {
	for _, mode := range []int32{1, 2, 3, 4, 5, 6, 7, 10} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			f := newTrackingFixture(t)
			if mode == 10 {
				f.setInfo(t, "uploading", 1, 100, 0)
			}
			f.mode.Store(mode)
			f.poll(t)
			current := f.task(t).Download
			if current.ErrorCode == nil || current.Handoff != nil || f.adds.Load() != 1 {
				t.Fatal(current)
			}
			if mode == 5 || mode == 6 {
				if current.State != "unavailable" || current.PollState != "queued" {
					t.Fatal(current)
				}
				f.check(t, 409)
				return
			}
			if current.State != "needs-review" || current.PollState != "idle" {
				t.Fatal(current)
			}
			f.mode.Store(0)
			f.setInfo(t, "downloading", .25, 100, 75)
			queued := f.check(t, 202)
			f.poll(t)
			if f.task(t).Download.State != "downloading" || queued.Revision <= current.Revision {
				t.Fatal("explicit check failed")
			}
		})
	}
	f := newTrackingFixture(t)
	id := f.ready.ID
	for _, path := range []string{"/api/v1/downloads?limit=51", "/api/v1/downloads?offset=-1", "/api/v1/downloads?unexpected=1", "/api/v1/downloads?limit=1&limit=2"} {
		if r := f.request(t, "GET", path, ""); r.Code != 422 {
			t.Fatal(path, r.Code)
		}
	}
	for _, body := range []string{`{}`, `{"revision":0}`, `{"revision":null}`, `{"revision":1,"revision":1}`, `{"revision":1,"url":"http://other"}`} {
		r := f.request(t, "POST", "/api/v1/downloads/"+id+"/check", body)
		if r.Code != 400 && r.Code != 422 {
			t.Fatal(body, r.Code)
		}
	}
	before := f.calls.Load()
	if r := f.request(t, "GET", "/api/v1/downloads/"+strings.Repeat("a", 32), ""); r.Code != 404 {
		t.Fatal(r.Code)
	}
	if f.calls.Load() != before {
		t.Fatal("malformed API contacted client")
	}
}
func TestDownloadCompletionEvidenceAndAdversarialFiles(t *testing.T) {
	cases := []struct {
		name          string
		files         []map[string]any
		state, reason string
	}{
		{"multiple", []map[string]any{remoteEvidence("Movie/film.mkv", 60, 1, 1, 0), remoteEvidence("Movie/sample.mkv", 40, 1, 1, 1)}, "completed", "download-file-ambiguous"},
		{"no-video", []map[string]any{remoteEvidence("Movie/archive.rar", 100, 1, 1, 0)}, "completed", "download-file-ambiguous"},
		{"incomplete", []map[string]any{remoteEvidence("Movie/film.mkv", 100, .8, 1, 0)}, "completed", "download-files-incomplete"},
		{"size-mismatch", []map[string]any{remoteEvidence("Movie/film.mkv", 90, 1, 1, 0)}, "completed", "download-files-incomplete"},
		{"traversal", []map[string]any{remoteEvidence("../outside.mkv", 100, 1, 1, 0)}, "needs-review", "download-file-path-unsafe"},
		{"drive", []map[string]any{remoteEvidence(`C:\outside.mkv`, 100, 1, 1, 0)}, "needs-review", "download-file-path-unsafe"},
		{"unc", []map[string]any{remoteEvidence(`\\host\film.mkv`, 100, 1, 1, 0)}, "needs-review", "download-file-path-unsafe"},
		{"reserved", []map[string]any{remoteEvidence("Movie/CON.mkv", 100, 1, 1, 0)}, "needs-review", "download-file-path-unsafe"},
		{"collision", []map[string]any{remoteEvidence("Movie/film.mkv", 50, 1, 1, 0), remoteEvidence("Movie/FILM.mkv", 50, 1, 1, 1)}, "needs-review", "download-file-path-unsafe"},
		{"duplicate-index", []map[string]any{remoteEvidence("Movie/a.mkv", 50, 1, 1, 0), remoteEvidence("Movie/b.mkv", 50, 1, 1, 0)}, "needs-review", "download-client-invalid-response"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newTrackingFixture(t)
			f.setInfo(t, "uploading", 1, 100, 0)
			f.setFiles(t, c.files)
			f.poll(t)
			d := f.task(t).Download
			code := d.ErrorCode
			if d.Handoff != nil {
				code = d.Handoff.ReasonCode
			}
			if d.State != c.state || code == nil || *code != c.reason {
				t.Fatal(d)
			}
		})
	}
	t.Run("unselected", func(t *testing.T) {
		f := newTrackingFixture(t)
		f.setInfo(t, "stoppedUP", 1, 100, 0)
		f.setFiles(t, []map[string]any{remoteEvidence("Movie/video.mkv", 100, 1, 1, 0), remoteEvidence("Movie/other.mkv", 50, 0, 0, 1)})
		f.poll(t)
		if d := f.task(t).Download; d.Handoff == nil || d.Handoff.State != "awaiting-validation" {
			t.Fatal(d)
		}
	})
	t.Run("race", func(t *testing.T) {
		f := newTrackingFixture(t)
		f.setInfo(t, "uploading", 1, 100, 0)
		f.mode.Store(9)
		f.poll(t)
		if d := f.task(t).Download; d.State != "needs-review" || d.Handoff != nil || *d.ErrorCode != "download-completion-changed" {
			t.Fatal(d)
		}
	})
}
func TestDownloadCompletionStateAndConfigurationFences(t *testing.T) {
	for _, c := range []struct {
		client, state string
		progress      float64
		left          int64
		fileReads     int32
	}{{"checkingUP", "downloading", 1, 0, 0}, {"moving", "downloading", 1, 0, 0}, {"unknown", "needs-review", 0, 100, 0}, {"error", "failed", 0, 100, 0}, {"missingFiles", "failed", 0, 100, 0}, {"uploading", "needs-review", .9, 1, 0}} {
		t.Run(c.client, func(t *testing.T) {
			f := newTrackingFixture(t)
			f.setInfo(t, c.client, c.progress, 100, c.left)
			f.poll(t)
			d := f.task(t).Download
			if d.State != c.state || f.fileReads.Load() != c.fileReads || d.Handoff != nil {
				t.Fatal(d)
			}
		})
	}
	f := newTrackingFixture(t)
	f.mode.Store(8)
	done := make(chan error, 1)
	go func() { _, err := (&downloads.Engine{Repo: f.store}).Step(context.Background()); done <- err }()
	select {
	case <-f.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("poll did not begin")
	}
	cfg, _ := f.store.GetSettings(context.Background())
	if _, err := f.store.UpdateSettings(context.Background(), cfg.Revision, settings.Input{Directories: cfg.Directories, Profile: cfg.Profile}); err != nil {
		t.Fatal(err)
	}
	close(f.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	d := f.task(t).Download
	if d.State != "needs-review" || d.Handoff != nil || *d.ErrorCode != "submission-configuration-changed" {
		t.Fatal(d)
	}
	f.check(t, 409)
	if f.adds.Load() != 1 {
		t.Fatal("configuration conflict resubmitted")
	}
}

func TestDownloadRejectsMalformedMetricsAndPreAuthenticationClientChange(t *testing.T) {
	for _, c := range []struct {
		name, field string
		value       any
	}{{"missing-state", "state", nil}, {"negative-progress", "progress", -.1}, {"oversized-progress", "progress", 1.1}, {"negative-size", "size", -1}, {"noninteger-size", "size", 1.5}, {"unsafe-integer", "size", int64(9007199254740992)}, {"remaining-over-size", "amount_left", 101}, {"negative-speed", "dlspeed", -1}, {"wrong-type", "progress", "1"}} {
		t.Run(c.name, func(t *testing.T) {
			f := newTrackingFixture(t)
			var rows []map[string]any
			json.Unmarshal([]byte(f.infoBody.Load().(string)), &rows)
			rows[0][c.field] = c.value
			data, _ := json.Marshal(rows)
			f.infoBody.Store(string(data))
			f.poll(t)
			d := f.task(t).Download
			if d.State != "needs-review" || d.Handoff != nil || d.LastObservedAt != nil {
				t.Fatal(d)
			}
		})
	}
	f := newTrackingFixture(t)
	before := f.calls.Load()
	client, _ := f.store.GetDownloadClient(context.Background())
	if _, err := f.store.SaveDownloadClient(context.Background(), client.Revision, client.URL, client.Username, "dummy-replacement"); err != nil {
		t.Fatal(err)
	}
	f.poll(t)
	d := f.task(t).Download
	if d.State != "needs-review" || *d.ErrorCode != "download-client-changed" || f.calls.Load() != before {
		t.Fatal("stale client contacted", d, f.calls.Load(), before)
	}
}
