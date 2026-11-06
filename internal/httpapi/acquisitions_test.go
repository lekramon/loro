package httpapi

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lekramon/piratarr/internal/acquisitions"
	"github.com/lekramon/piratarr/internal/catalog"
	"github.com/lekramon/piratarr/internal/settings"
	"github.com/lekramon/piratarr/internal/sources"
	"github.com/lekramon/piratarr/internal/storage"
)

const selectionHash = "0123456789abcdef0123456789abcdef01234567"
const torrentInfo = `d6:lengthi1e4:name9:movie.mkv12:piece lengthi16384e6:pieces20:00000000000000000000e`
const torrentFixture = `d8:announce39:http://private.invalid/dummy-source-key4:info` + torrentInfo + `e`

type acquisitionFixture struct {
	*catalogFixture
	server                           *httptest.Server
	reference, title, claim, payload atomic.Value
	mode, fetches, searches          atomic.Int32
	entered, release                 chan struct{}
	movie                            catalog.Movie
}

func newAcquisitionFixture(t *testing.T) *acquisitionFixture {
	f := &acquisitionFixture{catalogFixture: newCatalogFixture(t, true), entered: make(chan struct{}, 1), release: make(chan struct{})}
	f.reference.Store("magnet:?xt=urn:btih:" + selectionHash + "&tr=http%3A%2F%2Fprivate.invalid%2Fdummy-source-key")
	f.title.Store("Movie.1080p.Remux.mkv")
	f.claim.Store("")
	f.payload.Store([]byte(torrentFixture))
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/download" {
			f.fetches.Add(1)
			if r.URL.Query().Get("token") != "private-reference" || r.URL.Query().Has("apikey") {
				t.Error("reference altered or credentials appended")
			}
			switch f.mode.Load() {
			case 401, 429, 503:
				w.WriteHeader(int(f.mode.Load()))
				fmt.Fprint(w, "dummy-source-key")
				return
			case 1:
				w.Header().Set("Location", f.server.URL+"/unexpected")
				w.WriteHeader(302)
				return
			case 2:
				select {
				case f.entered <- struct{}{}:
				default:
				}
				select {
				case <-f.release:
				case <-r.Context().Done():
					return
				}
			case 3:
				fmt.Fprint(w, strings.Repeat("x", (4<<20)+1))
				return
			case 4:
				fmt.Fprint(w, "<html>private dummy-source-key</html>")
				return
			}
			w.Write(f.payload.Load().([]byte))
			return
		}
		if r.URL.Path != "/api" {
			t.Error("unexpected fetch, redirect or client submission", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		if r.URL.Query().Get("t") == "caps" {
			fmt.Fprint(w, sourceCaps)
			return
		}
		f.searches.Add(1)
		fmt.Fprintf(w, `<rss xmlns:t="http://torznab.com/schemas/2015/feed"><channel><item><title>%s</title><guid>private-selection-guid</guid><enclosure url="%s"/><t:attr name="language" value="French"/><t:attr name="infohash" value="%s"/></item></channel></rss>`, html.EscapeString(f.title.Load().(string)), html.EscapeString(f.reference.Load().(string)), html.EscapeString(f.claim.Load().(string)))
	}))
	t.Cleanup(f.server.Close)
	decodeResponse[sources.Public](t, f.request(t, "PUT", "/api/v1/source", fmt.Sprintf(`{"revision":0,"name":"Source","url":%q,"apiKey":%q}`, f.server.URL+"/api", sourceKey)), 200)
	decodeResponse[sources.TestResult](t, f.request(t, "GET", "/api/v1/source/test", ""), 200)
	registered := decodeResponse[struct {
		Movie catalog.Movie `json:"movie"`
	}](t, f.request(t, "POST", "/api/v1/catalog/movies", `{"tmdbId":42,"language":"en-US"}`), 201)
	f.movie = registered.Movie
	return f
}
func (f *acquisitionFixture) observe(t *testing.T) sources.Candidate {
	r := f.request(t, "GET", fmt.Sprintf("/api/v1/releases?movieId=%d", f.movie.ID), "")
	page := decodeResponse[sources.ReleasePage](t, r, 200)
	if len(page.Items) != 1 || !acquisitions.ValidID(page.Items[0].ObservationID) {
		t.Fatal(page)
	}
	if strings.Contains(r.Body.String(), "dummy-source-key") || strings.Contains(r.Body.String(), "private-reference") {
		t.Fatal("private observation leaked")
	}
	return page.Items[0]
}

type selectionResponse struct {
	Acquisition acquisitions.Acquisition `json:"acquisition"`
	Created     bool                     `json:"created"`
}

func (f *acquisitionFixture) selectCandidate(t *testing.T, candidate sources.Candidate, key string, confirm bool, status int) selectionResponse {
	return decodeResponse[selectionResponse](t, f.request(t, "POST", "/api/v1/acquisitions", fmt.Sprintf(`{"observationId":%q,"idempotencyKey":%q,"confirmOverride":%t}`, candidate.ObservationID, key, confirm)), status)
}
func (f *acquisitionFixture) action(t *testing.T, value acquisitions.Acquisition, action string) *httptest.ResponseRecorder {
	return f.request(t, "POST", "/api/v1/acquisitions/"+value.ID+"/"+action, fmt.Sprintf(`{"revision":%d}`, value.Revision))
}

func TestAcquisitionDurableMagnetSelectionReplayAndPrivateHistory(t *testing.T) {
	f := newAcquisitionFixture(t)
	before, _ := f.store.GetSettings(context.Background())
	candidate := f.observe(t)
	selected := f.selectCandidate(t, candidate, "selection-request-key-001", false, 201)
	if !selected.Created || selected.Acquisition.State != "selected" || selected.Acquisition.OverrideConfirmed || f.fetches.Load() != 0 {
		t.Fatal(selected)
	}
	ready := decodeResponse[acquisitions.Acquisition](t, f.action(t, selected.Acquisition, "prepare"), 200)
	if ready.State != "ready" || ready.InfoHash == nil || *ready.InfoHash != selectionHash || ready.Kind == nil || *ready.Kind != "magnet" || f.fetches.Load() != 0 {
		t.Fatal(ready)
	}
	f.handler.acquisitions.Now = func() time.Time { return time.Now().Add(time.Hour) }
	replay := f.selectCandidate(t, candidate, "selection-request-key-001", false, 200)
	if replay.Acquisition.ID != ready.ID || replay.Acquisition.State != "ready" {
		t.Fatal(replay)
	}
	f.handler.acquisitions.Now = time.Now
	newObservation := f.observe(t)
	duplicate := f.selectCandidate(t, newObservation, "selection-request-key-002", false, 200)
	if duplicate.Created || duplicate.Acquisition.ID != ready.ID {
		t.Fatal(duplicate)
	}
	listResponse := f.request(t, "GET", "/api/v1/acquisitions", "")
	list := decodeResponse[acquisitions.List](t, listResponse, 200)
	if list.Total != 1 || len(list.Items) != 1 {
		t.Fatal(list)
	}
	for _, private := range []string{"magnet:", "dummy-source-key", "private-selection-guid", f.server.URL, "private-reference"} {
		if strings.Contains(listResponse.Body.String(), private) {
			t.Fatal("private history exposed", private)
		}
	}
	after, _ := f.store.GetSettings(context.Background())
	if !reflect.DeepEqual(before, after) {
		t.Fatal("selection mutated preferences")
	}
	if err := f.store.Close(); err != nil {
		t.Fatal(err)
	}
	var err error
	f.store, err = storage.Open(context.Background(), f.data)
	if err != nil {
		t.Fatal(err)
	}
	f.rebind()
	restored := decodeResponse[acquisitions.Acquisition](t, f.request(t, "GET", "/api/v1/acquisitions/"+ready.ID, ""), 200)
	if !reflect.DeepEqual(restored, ready) {
		t.Fatal("intent changed at restart")
	}
	cancelled := decodeResponse[acquisitions.Acquisition](t, f.action(t, ready, "cancel"), 200)
	if cancelled.State != "cancelled" {
		t.Fatal(cancelled)
	}
	newSelection := f.selectCandidate(t, f.observe(t), "selection-request-key-003", false, 201)
	if newSelection.Acquisition.ID == ready.ID {
		t.Fatal("cancel did not release duplicate exclusion")
	}
}

func TestAcquisitionConfirmationExpiryAndRevisionChecks(t *testing.T) {
	f := newAcquisitionFixture(t)
	f.title.Store("Unknown quality movie")
	candidate := f.observe(t)
	r := f.request(t, "POST", "/api/v1/acquisitions", fmt.Sprintf(`{"observationId":%q,"idempotencyKey":"confirm-selection-key-01","confirmOverride":false}`, candidate.ObservationID))
	if r.Code != 409 || !strings.Contains(r.Body.String(), "profile-confirmation-required") {
		t.Fatal(r.Code, r.Body.String())
	}
	confirmed := f.selectCandidate(t, candidate, "confirm-selection-key-01", true, 201)
	if !confirmed.Acquisition.OverrideConfirmed {
		t.Fatal(confirmed)
	}
	r = f.request(t, "POST", "/api/v1/acquisitions", fmt.Sprintf(`{"observationId":%q,"idempotencyKey":"confirm-selection-key-01","confirmOverride":false}`, candidate.ObservationID))
	if r.Code != 409 || !strings.Contains(r.Body.String(), "idempotency-conflict") {
		t.Fatal(r.Code, r.Body.String())
	}
	f.handler.acquisitions.Now = func() time.Time { return time.Now().Add(31 * time.Minute) }
	r = f.request(t, "POST", "/api/v1/acquisitions", fmt.Sprintf(`{"observationId":%q,"idempotencyKey":"expired-selection-key-01","confirmOverride":true}`, candidate.ObservationID))
	if r.Code != 409 || !strings.Contains(r.Body.String(), "observation-expired") {
		t.Fatal(r.Code, r.Body.String())
	}
	f.handler.acquisitions.Now = time.Now
	fresh := f.observe(t)
	profile, _ := f.store.GetSettings(context.Background())
	profile.Profile.PreferredResolution = "720p"
	_, err := f.store.UpdateSettings(context.Background(), profile.Revision, settings.Input{Directories: profile.Directories, Profile: profile.Profile})
	if err != nil {
		t.Fatal(err)
	}
	r = f.request(t, "POST", "/api/v1/acquisitions", fmt.Sprintf(`{"observationId":%q,"idempotencyKey":"changed-selection-key-01","confirmOverride":true}`, fresh.ObservationID))
	if r.Code != 409 || !strings.Contains(r.Body.String(), "observation-changed") {
		t.Fatal(r.Code, r.Body.String())
	}
	ready := decodeResponse[acquisitions.Acquisition](t, f.action(t, confirmed.Acquisition, "prepare"), 200)
	if ready.State != "ready" || ready.ProfileRevision != 1 {
		t.Fatal("later preferences altered intent", ready)
	}
	if r = f.action(t, confirmed.Acquisition, "cancel"); r.Code != 409 {
		t.Fatal("stale revision accepted", r.Code)
	}
}

func TestAcquisitionHTTPMetainfoAndReviewFailures(t *testing.T) {
	f := newAcquisitionFixture(t)
	f.reference.Store(f.server.URL + "/download?token=private-reference")
	selected := f.selectCandidate(t, f.observe(t), "torrent-selection-key-01", false, 201).Acquisition
	response := f.action(t, selected, "prepare")
	ready := decodeResponse[acquisitions.Acquisition](t, response, 200)
	expected := sha1.Sum([]byte(torrentInfo))
	if ready.State != "ready" || ready.InfoHash == nil || *ready.InfoHash != hex.EncodeToString(expected[:]) || f.fetches.Load() != 1 {
		t.Fatal(ready)
	}
	stored, err := f.store.GetAcquisition(context.Background(), ready.ID)
	if err != nil || string(stored.Payload) != torrentFixture {
		t.Fatal("metainfo not preserved", err)
	}
	for _, private := range []string{"private-reference", "dummy-source-key", "announce"} {
		if strings.Contains(response.Body.String(), private) {
			t.Fatal("metainfo/private URL exposed")
		}
	}
	decodeResponse[acquisitions.Acquisition](t, f.action(t, ready, "cancel"), 200)
	for _, entry := range []struct {
		mode int32
		code string
	}{{401, "source-authentication-failed"}, {429, "source-rate-limited"}, {503, "source-unavailable"}, {1, "unsupported-reference"}, {3, "torrent-too-large"}, {4, "torrent-invalid"}} {
		f.mode.Store(entry.mode)
		selected = f.selectCandidate(t, f.observe(t), fmt.Sprintf("torrent-review-key-%03d", entry.mode), false, 201).Acquisition
		value := decodeResponse[acquisitions.Acquisition](t, f.action(t, selected, "prepare"), 200)
		if value.State != "needs-review" || value.ReviewCode == nil || *value.ReviewCode != entry.code {
			t.Fatal(value)
		}
		f.mode.Store(0)
		retried := decodeResponse[acquisitions.Acquisition](t, f.action(t, value, "prepare"), 200)
		if retried.State != "ready" {
			t.Fatal(retried)
		}
		decodeResponse[acquisitions.Acquisition](t, f.action(t, retried, "cancel"), 200)
	}
}

func TestAcquisitionRejectsCrossOriginAndIdentityMismatchWithoutClientCalls(t *testing.T) {
	f := newAcquisitionFixture(t)
	var foreignCalls atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { foreignCalls.Add(1) }))
	defer foreign.Close()
	for i, entry := range []struct{ reference, claim, code string }{{foreign.URL + "/file", "", "unsupported-reference"}, {"magnet:?xt=urn:btmh:1220abcdef", "", "unsupported-identity"}, {"magnet:?xt=urn:btih:" + selectionHash, strings.Repeat("0", 40), "identity-mismatch"}} {
		f.reference.Store(entry.reference)
		f.claim.Store(entry.claim)
		selected := f.selectCandidate(t, f.observe(t), fmt.Sprintf("identity-review-key-%03d", i), false, 201).Acquisition
		value := decodeResponse[acquisitions.Acquisition](t, f.action(t, selected, "prepare"), 200)
		if value.ReviewCode == nil || *value.ReviewCode != entry.code {
			t.Fatal(value)
		}
		decodeResponse[acquisitions.Acquisition](t, f.action(t, value, "cancel"), 200)
	}
	if foreignCalls.Load() != 0 || f.fetches.Load() != 0 {
		t.Fatal("unsupported reference caused a request")
	}
}

func TestAcquisitionPreparationFencesCancellationAndSourceChanges(t *testing.T) {
	for _, action := range []string{"cancel", "source"} {
		t.Run(action, func(t *testing.T) {
			f := newAcquisitionFixture(t)
			f.reference.Store(f.server.URL + "/download?token=private-reference")
			selected := f.selectCandidate(t, f.observe(t), "inflight-selection-key-01", false, 201).Acquisition
			f.mode.Store(2)
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() { done <- f.action(t, selected, "prepare") }()
			select {
			case <-f.entered:
			case <-time.After(time.Second):
				t.Fatal("fetch did not start")
			}
			inflight := decodeResponse[acquisitions.Acquisition](t, f.request(t, "GET", "/api/v1/acquisitions/"+selected.ID, ""), 200)
			if inflight.State != "preparing" {
				t.Fatal(inflight)
			}
			if r := f.action(t, inflight, "prepare"); r.Code != 409 {
				t.Fatal("overlapping preparation accepted", r.Code)
			}
			if action == "cancel" {
				decodeResponse[acquisitions.Acquisition](t, f.action(t, inflight, "cancel"), 200)
			} else {
				decodeResponse[sources.Public](t, f.request(t, "PUT", "/api/v1/source", fmt.Sprintf(`{"revision":1,"name":"Renamed","url":%q}`, f.server.URL+"/api")), 200)
			}
			close(f.release)
			select {
			case r := <-done:
				if action == "cancel" {
					if r.Code != 409 {
						t.Fatal(r.Code)
					}
				} else {
					value := decodeResponse[acquisitions.Acquisition](t, r, 200)
					if value.State != "needs-review" || value.ReviewCode == nil || *value.ReviewCode != "source-changed" {
						t.Fatal(value)
					}
				}
			case <-time.After(time.Second):
				t.Fatal("preparation did not finish")
			}
		})
	}
}

func TestAcquisitionCancelledRequestRecordsReviewAndReleasesPreparationGate(t *testing.T) {
	f := newAcquisitionFixture(t)
	f.reference.Store(f.server.URL + "/download?token=private-reference")
	selected := f.selectCandidate(t, f.observe(t), "cancelled-request-key-001", false, 201).Acquisition
	f.mode.Store(2)
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/v1/acquisitions/"+selected.ID+"/prepare", strings.NewReader(`{"revision":1}`)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Piratarr-Request", "1")
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { r := httptest.NewRecorder(); f.handler.ServeHTTP(r, request); done <- r }()
	select {
	case <-f.entered:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("request did not begin")
	}
	cancel()
	select {
	case r := <-done:
		value := decodeResponse[acquisitions.Acquisition](t, r, 200)
		if value.State != "needs-review" || value.ReviewCode == nil || *value.ReviewCode != "source-timeout" {
			t.Fatal(value)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled read blocked")
	}
	value := decodeResponse[acquisitions.Acquisition](t, f.request(t, "GET", "/api/v1/acquisitions/"+selected.ID, ""), 200)
	close(f.release)
	f.mode.Store(0)
	ready := decodeResponse[acquisitions.Acquisition](t, f.action(t, value, "prepare"), 200)
	if ready.State != "ready" {
		t.Fatal("preparation gate not released", ready)
	}
}

func TestAcquisitionStrictRequestsAndConcurrentIdempotentSelection(t *testing.T) {
	f := newAcquisitionFixture(t)
	candidate := f.observe(t)
	for _, body := range []string{`null`, `[]`, `{"observationId":null}`, `{"unknown":1}`, `{"observationId":"a","observationId":"b"}`, `{} {}`} {
		r := f.request(t, "POST", "/api/v1/acquisitions", body)
		if r.Code != 400 {
			t.Fatal(body, r.Code)
		}
	}
	if r := f.request(t, "POST", "/api/v1/acquisitions", `{}`); r.Code != 422 {
		t.Fatal(r.Code)
	}
	if r := f.request(t, "POST", "/api/v1/acquisitions", `{"observationId":"11111111111111111111111111111111","idempotencyKey":"unknown-observation-key-01","confirmOverride":false}`); r.Code != 404 {
		t.Fatal(r.Code)
	}
	for _, path := range []string{"/api/v1/acquisitions?limit=51", "/api/v1/acquisitions?offset=-1", "/api/v1/acquisitions?movieId=0", "/api/v1/acquisitions?unknown=1"} {
		if r := f.request(t, "GET", path, ""); r.Code != 422 {
			t.Fatal(path, r.Code)
		}
	}
	results := make(chan *httptest.ResponseRecorder, 2)
	for _, key := range []string{"concurrent-selection-key-01", "concurrent-selection-key-02"} {
		go func(key string) {
			results <- f.request(t, "POST", "/api/v1/acquisitions", fmt.Sprintf(`{"observationId":%q,"idempotencyKey":%q,"confirmOverride":false}`, candidate.ObservationID, key))
		}(key)
	}
	var selected acquisitions.Acquisition
	created := 0
	for i := 0; i < 2; i++ {
		r := <-results
		if r.Code != 200 && r.Code != 201 {
			t.Fatal(r.Code, r.Body.String())
		}
		if r.Code == 201 {
			created++
		}
		value := decodeResponse[selectionResponse](t, r, r.Code)
		if selected.ID != "" && value.Acquisition.ID != selected.ID {
			t.Fatal("concurrent selections duplicated intent")
		}
		selected = value.Acquisition
	}
	if created != 1 || f.fetches.Load() != 0 {
		t.Fatal("unexpected selection side effects", created)
	}
	request := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/v1/acquisitions/"+selected.ID+"/prepare", strings.NewReader(`{"revision":1}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	f.handler.ServeHTTP(recorder, request)
	if recorder.Code != 403 {
		t.Fatal("mutation header bypassed", recorder.Code)
	}
	for _, body := range []string{`{"revision":null}`, `{"revision":1,"extra":true}`, `{"revision":1,"revision":1}`} {
		if r := f.request(t, "POST", "/api/v1/acquisitions/"+selected.ID+"/prepare", body); r.Code != 400 {
			t.Fatal(body, r.Code)
		}
	}
	decodeResponse[sources.Public](t, f.request(t, "PUT", "/api/v1/source", fmt.Sprintf(`{"revision":1,"name":"Changed","url":%q}`, f.server.URL+"/api")), 200)
	value := decodeResponse[acquisitions.Acquisition](t, f.action(t, selected, "prepare"), 200)
	if value.ReviewCode == nil || *value.ReviewCode != "source-changed" || f.fetches.Load() != 0 {
		t.Fatal("stale source was fetched", value)
	}
}
