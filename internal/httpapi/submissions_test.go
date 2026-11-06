package httpapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lekramon/piratarr/internal/acquisitions"
	"github.com/lekramon/piratarr/internal/downloadclient"
	"github.com/lekramon/piratarr/internal/settings"
	"github.com/lekramon/piratarr/internal/sources"
	"github.com/lekramon/piratarr/internal/storage"
	"github.com/lekramon/piratarr/internal/submissions"
)

type submissionFixture struct {
	*acquisitionFixture
	qb                                          *httptest.Server
	mode, adds, calls, logins, logouts, creates atomic.Int32
	owned, category                             atomic.Bool
	ready                                       acquisitions.Acquisition
	entered, release                            chan struct{}
}

func newSubmissionFixture(t *testing.T, torrent bool) *submissionFixture {
	f := &submissionFixture{acquisitionFixture: newAcquisitionFixture(t), entered: make(chan struct{}, 1), release: make(chan struct{})}
	if torrent {
		f.reference.Store(f.server.URL + "/download?token=private-reference")
	}
	choice := f.selectCandidate(t, f.observe(t), "submission-selection-key", false, 201)
	f.ready = decodeResponse[acquisitions.Acquisition](t, f.action(t, choice.Acquisition, "prepare"), 200)
	f.qb = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		if r.Header.Get("Origin") != f.qb.URL || r.Header.Get("Referer") != f.qb.URL+"/" {
			t.Error("missing client origin")
		}
		route := strings.TrimPrefix(r.URL.Path, "/api/v2/")
		if route != "auth/login" {
			if c, e := r.Cookie("SID"); e != nil || c.Value != "simulated-session" {
				t.Error("missing session")
			}
		}
		switch route {
		case "auth/login":
			f.logins.Add(1)
			r.ParseForm()
			if r.Form.Get("password") != "dummy-client-password" {
				t.Error("password altered")
			}
			if f.mode.Load() == 6 {
				fmt.Fprint(w, "Fails.")
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "simulated-session", Path: "/"})
			fmt.Fprint(w, "Ok.")
		case "auth/logout":
			f.logouts.Add(1)
		case "app/version":
			fmt.Fprint(w, "v5.1.0")
		case "app/webapiVersion":
			fmt.Fprint(w, "2.11.3")
		case "torrents/info":
			if r.URL.Query().Get("hashes") != *f.ready.InfoHash || r.URL.Query().Has("category") || r.URL.Query().Has("tag") {
				t.Error("lookup hid foreign torrents")
			}
			switch f.mode.Load() {
			case 4:
				fmt.Fprint(w, "null")
				return
			case 5:
				w.WriteHeader(503)
				return
			case 7:
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(429)
				return
			case 8:
				fmt.Fprint(w, strings.Repeat("x", 65537))
				return
			case 10:
				w.Header().Set("Location", f.qb.URL+"/unexpected")
				w.WriteHeader(302)
				return
			}
			if f.mode.Load() == 3 {
				fmt.Fprintf(w, `[{"hash":%q,"category":"other","tags":"someone-else"}]`, *f.ready.InfoHash)
				return
			}
			if f.owned.Load() {
				fmt.Fprintf(w, `[{"hash":%q,"category":"louro","tags":%q}]`, strings.ToUpper(*f.ready.InfoHash), "other, louro-acquisition-"+f.ready.ID)
				return
			}
			fmt.Fprint(w, "[]")
		case "torrents/categories":
			if f.category.Load() {
				fmt.Fprint(w, `{"louro":{"name":"louro","savePath":""}}`)
			} else {
				fmt.Fprint(w, `{}`)
			}
		case "torrents/createCategory":
			r.ParseForm()
			if r.Form.Get("category") != "louro" {
				t.Error("category")
			}
			f.creates.Add(1)
			f.category.Store(true)
		case "torrents/add":
			f.adds.Add(1)
			if err := r.ParseMultipartForm(5 << 20); err != nil {
				t.Error(err)
				return
			}
			defer r.MultipartForm.RemoveAll()
			settings, _ := f.store.GetSettings(context.Background())
			if r.FormValue("category") != "louro" || r.FormValue("tags") != "louro-acquisition-"+f.ready.ID || r.FormValue("autoTMM") != "false" || r.FormValue("savepath") != settings.Directories.DownloadDirectory || r.FormValue("skip_checking") != "false" {
				t.Error("submission policy altered")
			}
			if torrent {
				file, _, err := r.FormFile("torrents")
				if err != nil {
					t.Error(err)
					return
				}
				data, _ := io.ReadAll(file)
				file.Close()
				if string(data) != torrentFixture {
					t.Error("metainfo altered")
				}
				if r.FormValue("urls") != "" {
					t.Error("provider URL forwarded")
				}
			} else if r.FormValue("urls") != f.reference.Load().(string) {
				t.Error("magnet altered")
			}
			if f.mode.Load() == 9 {
				f.entered <- struct{}{}
				select {
				case <-f.release:
				case <-r.Context().Done():
					return
				}
			}
			if f.mode.Load() != 2 {
				f.owned.Store(true)
			}
			if f.mode.Load() == 1 {
				conn, _, _ := w.(http.Hijacker).Hijack()
				conn.Close()
				return
			}
			fmt.Fprint(w, "Ok.")
		default:
			t.Error("unexpected mutation", route)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(f.qb.Close)
	decodeResponse[downloadclient.Public](t, f.request(t, "PUT", "/api/v1/download-client", fmt.Sprintf(`{"revision":0,"url":%q,"username":"admin","password":"dummy-client-password"}`, f.qb.URL)), 200)
	return f
}
func (f *submissionFixture) queue(t *testing.T) acquisitions.Acquisition {
	return decodeResponse[acquisitions.Acquisition](t, f.request(t, "POST", "/api/v1/acquisitions/"+f.ready.ID+"/submit", fmt.Sprintf(`{"revision":%d,"clientRevision":1,"settingsRevision":1}`, f.ready.Revision)), 202)
}
func (f *submissionFixture) current(t *testing.T) acquisitions.Acquisition {
	return decodeResponse[acquisitions.Acquisition](t, f.request(t, "GET", "/api/v1/acquisitions/"+f.ready.ID, ""), 200)
}
func (f *submissionFixture) step(t *testing.T) {
	t.Helper()
	worked, err := (&submissions.Engine{Repo: f.store}).Step(context.Background())
	if !worked || err != nil {
		t.Fatal(worked, err)
	}
}
func TestSubmissionQueueIdentityOwnershipAndLostAddResponse(t *testing.T) {
	for _, torrent := range []bool{false, true} {
		t.Run(fmt.Sprint(torrent), func(t *testing.T) {
			f := newSubmissionFixture(t, torrent)
			f.mode.Store(1)
			if torrent {
				f.category.Store(true)
			}
			queued := f.queue(t)
			if f.calls.Load() != 0 || queued.Submission == nil || queued.Submission.State != "queued" {
				t.Fatal("queue performed networking", queued)
			}
			decodeResponse[apiError](t, f.action(t, queued, "cancel"), 409)
			decodeResponse[apiError](t, f.action(t, queued, "prepare"), 409)
			// Current-revision replay is inert; old lost-response replay conflicts and is read back.
			decodeResponse[apiError](t, f.request(t, "POST", "/api/v1/acquisitions/"+queued.ID+"/submit", fmt.Sprintf(`{"revision":%d,"clientRevision":1,"settingsRevision":1}`, f.ready.Revision)), 409)
			decodeResponse[acquisitions.Acquisition](t, f.request(t, "POST", "/api/v1/acquisitions/"+queued.ID+"/submit", fmt.Sprintf(`{"revision":%d,"clientRevision":1,"settingsRevision":1}`, queued.Revision)), 202)
			f.step(t)
			current := f.current(t)
			if current.Submission.State != "submitted" || f.adds.Load() != 1 || f.logins.Load() != 1 || f.logouts.Load() != 1 || f.creates.Load() != map[bool]int32{true: 0, false: 1}[torrent] {
				t.Fatal(current.Submission, f.adds.Load())
			}
			history := f.request(t, "GET", "/api/v1/acquisitions", "")
			if strings.Contains(history.Body.String(), "dummy-") || strings.Contains(history.Body.String(), "magnet:") || strings.Contains(history.Body.String(), "lease") {
				t.Fatal("private data leaked")
			}
			// Another candidate with the same hash cannot acquire this transfer.
			f.title.Store("Different.1080p.Remux.mkv")
			f.claim.Store("")
			f.reference.Store("magnet:?xt=urn:btih:" + *f.ready.InfoHash)
			f.catalogFixture.handler.source = nil
			// Change source revision to generate a distinct candidate identity, retaining the hash.
			config, _ := f.store.GetSource(context.Background())
			_, err := f.store.SaveSource(context.Background(), config.Revision, config.Name, config.URL, "")
			if err != nil {
				t.Fatal(err)
			}
			f.catalogFixture.rebind()
			decodeResponse[sources.TestResult](t, f.request(t, "GET", "/api/v1/source/test", ""), 200)
			other := f.selectCandidate(t, f.observe(t), "second-hash-selection", false, 201)
			prepared := decodeResponse[acquisitions.Acquisition](t, f.action(t, other.Acquisition, "prepare"), 200)
			conflict := decodeResponse[apiError](t, f.request(t, "POST", "/api/v1/acquisitions/"+prepared.ID+"/submit", fmt.Sprintf(`{"revision":%d,"clientRevision":1,"settingsRevision":1}`, prepared.Revision)), 409)
			if conflict.Code != "submission-hash-conflict" {
				t.Fatal(conflict)
			}
		})
	}
}
func TestSubmissionUncertainNeverResubmitsAndRecoversRestart(t *testing.T) {
	f := newSubmissionFixture(t, false)
	f.mode.Store(2)
	f.queue(t)
	f.step(t)
	current := f.current(t)
	if current.Submission.State != "uncertain" || f.adds.Load() != 1 {
		t.Fatal(current)
	}
	decodeResponse[acquisitions.Acquisition](t, f.action(t, current, "reconcile"), 202)
	// Simulate process death after claim; startup preserves add evidence and requeues reconciliation.
	job, err := f.store.ClaimSubmission(context.Background(), acquisitions.Identifier(), time.Now())
	if err != nil || job == nil || !job.Attempted {
		t.Fatal(job, err)
	}
	f.store.Close()
	f.store, err = storage.Open(context.Background(), f.data)
	if err != nil {
		t.Fatal(err)
	}
	f.rebind()
	f.mode.Store(0)
	f.step(t)
	current = f.current(t)
	if current.Submission.State != "uncertain" || f.adds.Load() != 1 {
		t.Fatal("absent evidence resubmitted", current)
	}
	f.owned.Store(true)
	decodeResponse[acquisitions.Acquisition](t, f.action(t, current, "reconcile"), 202)
	f.step(t)
	if f.current(t).Submission.State != "submitted" || f.adds.Load() != 1 {
		t.Fatal("reconciliation did not adopt exact ownership")
	}
}
func TestSubmissionReviewPreAddRetryAndCooldown(t *testing.T) {
	for _, mode := range []int32{3, 4, 5, 6, 7, 8, 10} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			f := newSubmissionFixture(t, false)
			f.mode.Store(mode)
			f.queue(t)
			f.step(t)
			current := f.current(t)
			if current.Submission.State != "needs-review" || current.Submission.ErrorCode == nil || f.adds.Load() != 0 {
				t.Fatal(current)
			}
			if mode == 7 {
				failure := decodeResponse[apiError](t, f.action(t, current, "reconcile"), 409)
				if failure.Code != "submission-cooldown" {
					t.Fatal(failure)
				}
				return
			}
			f.mode.Store(0)
			decodeResponse[acquisitions.Acquisition](t, f.action(t, current, "reconcile"), 202)
			f.step(t)
			if f.current(t).Submission.State != "submitted" || f.adds.Load() != 1 {
				t.Fatal("safe pre-add retry failed")
			}
		})
	}
}
func TestSubmissionFencesInFlightConfigurationChange(t *testing.T) {
	f := newSubmissionFixture(t, false)
	f.mode.Store(9)
	f.queue(t)
	finished := make(chan error, 1)
	go func() { _, err := (&submissions.Engine{Repo: f.store}).Step(context.Background()); finished <- err }()
	select {
	case <-f.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("add not reached")
	}
	c, _ := f.store.GetDownloadClient(context.Background())
	if _, err := f.store.SaveDownloadClient(context.Background(), c.Revision, c.URL, c.Username, "replacement"); err != nil {
		t.Fatal(err)
	}
	close(f.release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	current := f.current(t)
	if current.Submission.State != "needs-review" || *current.Submission.ErrorCode != "download-client-changed" {
		t.Fatal(current)
	}
	failure := decodeResponse[apiError](t, f.action(t, current, "reconcile"), 409)
	if failure.Code != "download-client-changed" || f.adds.Load() != 1 {
		t.Fatal(failure)
	}
}
func TestSubmissionStrictInputAndPreNetworkRevisionFence(t *testing.T) {
	f := newSubmissionFixture(t, false)
	base := "/api/v1/acquisitions/" + f.ready.ID
	for _, body := range []string{`{}`, `{"revision":1,"clientRevision":null}`, `{"revision":1,"clientRevision":1,"url":"http://other"}`, `{"revision":1,"revision":1,"clientRevision":1,"settingsRevision":1}`} {
		if r := f.request(t, "POST", base+"/submit", body); r.Code != 400 && r.Code != 422 {
			t.Fatal(r.Code, r.Body.String())
		}
	}
	queued := f.queue(t)
	input, _ := f.store.GetSettings(context.Background())
	if _, err := f.store.UpdateSettings(context.Background(), input.Revision, settings.Input{Directories: input.Directories, Profile: input.Profile}); err != nil {
		t.Fatal(err)
	}
	f.step(t)
	current := f.current(t)
	if current.Submission.State != "needs-review" || *current.Submission.ErrorCode != "submission-configuration-changed" || f.calls.Load() != 0 {
		t.Fatal(current, queued, f.calls.Load())
	}
}

func TestSubmissionWorkerPoolAndShutdown(t *testing.T) {
	f := newSubmissionFixture(t, false)
	f.queue(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); (&submissions.Engine{Repo: f.store}).Run(ctx, 4) }()
	deadline := time.Now().Add(5 * time.Second)
	for f.current(t).Submission.State != "submitted" {
		if time.Now().After(deadline) {
			t.Fatal("workers did not process queued intent")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("workers did not stop")
	}
	if f.adds.Load() != 1 {
		t.Fatal("worker overlap", f.adds.Load())
	}
}
