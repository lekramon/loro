package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lekramon/piratarr/internal/downloadclient"
	"github.com/lekramon/piratarr/internal/storage"
)

const clientPassword = " dummy-client-password &+ç "
const clientSID = "dummy-qbit-session"

type clientFixture struct {
	*catalogFixture
	server                          *httptest.Server
	mode                            atomic.Int32
	app, api, password              atomic.Value
	logins, versions, apis, logouts atomic.Int32
	entered, release                chan struct{}
}

func newClientFixture(t *testing.T) *clientFixture {
	f := &clientFixture{catalogFixture: newCatalogFixture(t, true), entered: make(chan struct{}, 1), release: make(chan struct{})}
	f.app.Store("v5.1.0")
	f.api.Store("2.11.3")
	f.password.Store("")
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != f.server.URL || r.Header.Get("Referer") != f.server.URL+"/qbit/" {
			t.Error("wrong qBittorrent Origin/Referer")
		}
		switch r.URL.Path {
		case "/qbit/api/v2/auth/login":
			f.logins.Add(1)
			if r.Method != "POST" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || r.Header.Get("Cookie") != "" {
				t.Error("invalid login or previous session reused")
			}
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			f.password.Store(r.Form.Get("password"))
			if r.Form.Get("username") == "" {
				t.Error("missing username")
			}
			switch f.mode.Load() {
			case 401, 403, 429, 500, 503:
				w.Header().Set("Retry-After", "2")
				w.WriteHeader(int(f.mode.Load()))
				fmt.Fprint(w, "private dummy-client-password")
				return
			case 1:
				fmt.Fprint(w, "Fails.")
				return
			case 2:
				fmt.Fprint(w, "Ok.")
				return
			case 3:
				w.Header().Set("Location", f.server.URL+"/unexpected")
				w.WriteHeader(302)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: clientSID, Path: "/qbit", HttpOnly: true})
			fmt.Fprint(w, "Ok.")
		case "/qbit/api/v2/app/version":
			f.versions.Add(1)
			if r.Method != "GET" {
				t.Error("version is not GET")
			}
			cookie, err := r.Cookie("SID")
			if err != nil || cookie.Value != clientSID {
				t.Error("missing authenticated cookie")
			}
			switch f.mode.Load() {
			case 4:
				fmt.Fprint(w, "<html>dummy-client-password</html>")
				return
			case 5:
				fmt.Fprint(w, strings.Repeat("x", 4097))
				return
			case 6:
				select {
				case f.entered <- struct{}{}:
				default:
				}
				select {
				case <-f.release:
				case <-r.Context().Done():
					return
				}
			case 7:
				<-r.Context().Done()
				return
			}
			fmt.Fprint(w, f.app.Load())
		case "/qbit/api/v2/app/webapiVersion":
			f.apis.Add(1)
			cookie, err := r.Cookie("SID")
			if err != nil || cookie.Value != clientSID {
				t.Error("missing API cookie")
			}
			fmt.Fprint(w, f.api.Load())
		case "/qbit/api/v2/auth/logout":
			f.logouts.Add(1)
			cookie, err := r.Cookie("SID")
			if r.Method != "POST" || err != nil || cookie.Value != clientSID {
				t.Error("invalid logout")
			}
			if f.mode.Load() == 8 {
				w.WriteHeader(500)
			}
		default:
			t.Error("unexpected endpoint, redirect or torrent mutation", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}
func (f *clientFixture) save(t *testing.T, revision int, password string) downloadclient.Public {
	body := fmt.Sprintf(`{"revision":%d,"url":%q,"username":"admin","password":%q}`, revision, f.server.URL+"/qbit/", password)
	response := f.request(t, "PUT", "/api/v1/download-client", body)
	if strings.Contains(response.Body.String(), "dummy-client-password") {
		t.Fatal("private password exposed")
	}
	return decodeResponse[downloadclient.Public](t, response, 200)
}
func (f *clientFixture) test(t *testing.T) *httptest.ResponseRecorder {
	return f.request(t, "POST", "/api/v1/download-client/test", `{}`)
}

func TestDownloadClientPrivateSaveFreshSessionAndRestart(t *testing.T) {
	f := newClientFixture(t)
	initial := decodeResponse[downloadclient.Public](t, f.request(t, "GET", "/api/v1/download-client", ""), 200)
	if initial.Configured || initial.Revision != 0 || initial.HasPassword || initial.AppVersion != nil {
		t.Fatal(initial)
	}
	first := f.save(t, 0, clientPassword)
	if !first.HasPassword || first.Revision != 1 || first.URL != f.server.URL+"/qbit" || f.logins.Load() != 0 {
		t.Fatal(first)
	}
	r := f.test(t)
	evidence := decodeResponse[downloadclient.TestResult](t, r, 200)
	if !evidence.Connected || evidence.ClientRevision != 1 || evidence.AppVersion != "v5.1.0" || evidence.APIVersion != "2.11.3" || evidence.CheckedAt == "" {
		t.Fatal(evidence)
	}
	if f.password.Load() != clientPassword || f.logins.Load() != 1 || f.logouts.Load() != 1 {
		t.Fatal("password modified or session not cleaned up")
	}
	for _, value := range []string{"dummy-client-password", clientSID} {
		if strings.Contains(r.Body.String(), value) {
			t.Fatal("credential exposed")
		}
	}
	decodeResponse[downloadclient.TestResult](t, f.test(t), 200)
	if f.logins.Load() != 2 || f.versions.Load() != 2 || f.logouts.Load() != 2 {
		t.Fatal("test reused evidence/session")
	}
	f.mode.Store(401)
	if r = f.test(t); r.Code != 503 {
		t.Fatal(r.Code)
	}
	saved := decodeResponse[downloadclient.Public](t, f.request(t, "GET", "/api/v1/download-client", ""), 200)
	if saved.AppVersion == nil || saved.CheckedAt == nil {
		t.Fatal("failed test erased last success")
	}
	if health := f.request(t, "GET", "/api/v1/health/ready", ""); health.Code != 200 {
		t.Fatal("remote outage affected readiness")
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
	f.mode.Store(0)
	decodeResponse[downloadclient.TestResult](t, f.test(t), 200)
	if f.password.Load() != clientPassword || f.logins.Load() != 4 {
		t.Fatal("private configuration not restored")
	}
	rotated := f.save(t, 1, "")
	if rotated.Revision != 2 || rotated.CheckedAt != nil {
		t.Fatal(rotated)
	}
	decodeResponse[downloadclient.TestResult](t, f.test(t), 200)
	if f.password.Load() != clientPassword {
		t.Fatal("blank erased password")
	}
	f.save(t, 2, "replacement-password")
	decodeResponse[downloadclient.TestResult](t, f.test(t), 200)
	if f.password.Load() != "replacement-password" {
		t.Fatal("password rotation failed")
	}
}

func TestDownloadClientRejectsUnsafeConfigurationAndMalformedBodies(t *testing.T) {
	f := newClientFixture(t)
	for _, body := range []string{`null`, `[]`, `{}`, `{"revision":0,"url":null}`, `{"revision":0,"revision":0}`, `{"unknown":true}`, `{"revision":"0"}`, `{"revision":0} {}`} {
		r := f.request(t, "PUT", "/api/v1/download-client", body)
		if r.Code != 400 && r.Code != 422 {
			t.Fatal(body, r.Code)
		}
	}
	for _, endpoint := range []string{"file:///tmp/test", "http://user:pass@localhost:8080", "http://localhost/api?key=private", "http://localhost#", "http://localhost?", "http://localhost:0", "http://localhost:70000", "http://localhost:"} {
		r := f.request(t, "PUT", "/api/v1/download-client", fmt.Sprintf(`{"revision":0,"url":%q,"username":"admin","password":"dummy"}`, endpoint))
		if r.Code != 422 {
			t.Fatal(endpoint, r.Code)
		}
	}
	if r := f.request(t, "PUT", "/api/v1/download-client", fmt.Sprintf(`{"revision":0,"url":%q,"username":"admin"}`, f.server.URL)); r.Code != 422 {
		t.Fatal("initial password optional", r.Code)
	}
	f.save(t, 0, clientPassword)
	for _, entry := range []struct{ endpoint, user string }{{f.server.URL + "/other", "admin"}, {f.server.URL + "/qbit", "other-user"}} {
		r := f.request(t, "PUT", "/api/v1/download-client", fmt.Sprintf(`{"revision":1,"url":%q,"username":%q}`, entry.endpoint, entry.user))
		if r.Code != 422 {
			t.Fatal("credential reused for changed identity", r.Code)
		}
	}
	if r := f.request(t, "PUT", "/api/v1/download-client", fmt.Sprintf(`{"revision":0,"url":%q,"username":"admin","password":"new"}`, f.server.URL)); r.Code != 409 {
		t.Fatal("stale save accepted", r.Code)
	}
	for _, body := range []string{"", `null`, `[]`, `{"revision":1}`, `{} {}`} {
		if r := f.request(t, "POST", "/api/v1/download-client/test", body); r.Code != 400 {
			t.Fatal(body, r.Code)
		}
	}
	if r := f.request(t, "GET", "/api/v1/download-client", "{}"); r.Code != 400 {
		t.Fatal(r.Code)
	}
	if r := f.request(t, "GET", "/api/v1/download-client?secret=1", ""); r.Code != 422 {
		t.Fatal(r.Code)
	}
	if f.logins.Load() != 0 {
		t.Fatal("save or invalid input contacted client")
	}
}

func TestDownloadClientClassifiesFailuresWithoutRetryOrCredentialLeak(t *testing.T) {
	for _, entry := range []struct {
		mode   int32
		status int
		code   string
	}{{1, 503, "authentication-failed"}, {2, 502, "invalid-response"}, {3, 502, "invalid-response"}, {4, 502, "invalid-response"}, {5, 502, "invalid-response"}, {7, 504, "timeout"}, {401, 503, "authentication-failed"}, {403, 503, "authentication-failed"}, {500, 503, "unavailable"}, {429, 429, "rate-limited"}} {
		t.Run(entry.code+fmt.Sprint(entry.mode), func(t *testing.T) {
			f := newClientFixture(t)
			f.save(t, 0, clientPassword)
			f.mode.Store(entry.mode)
			r := f.test(t)
			if r.Code != entry.status || !strings.Contains(r.Body.String(), "download-client-"+entry.code) || strings.Contains(r.Body.String(), "dummy-client-password") || strings.Contains(r.Body.String(), clientSID) {
				t.Fatal(r.Code, r.Body.String())
			}
			if f.logins.Load() != 1 {
				t.Fatal("login was retried")
			}
			if entry.mode == 4 || entry.mode == 5 || entry.mode == 7 {
				if f.logouts.Load() != 1 {
					t.Fatal("failed check did not attempt logout")
				}
			}
			if entry.mode == 429 {
				r = f.test(t)
				if r.Code != 429 || r.Header().Get("Retry-After") == "" || f.logins.Load() != 1 {
					t.Fatal("cooldown ignored")
				}
				f.save(t, 1, clientPassword)
				f.mode.Store(0)
				decodeResponse[downloadclient.TestResult](t, f.test(t), 200)
			}
		})
	}
}

func TestDownloadClientVersionBoundsAndLogoutIsBestEffort(t *testing.T) {
	f := newClientFixture(t)
	f.save(t, 0, clientPassword)
	for _, entry := range []struct {
		app, api string
		status   int
	}{{"v4.6.7", "2.11.3", 503}, {"v6.0.0", "2.11.3", 503}, {"v5.1.0", "3.0", 503}, {"v5.2.0beta1", "2.11.3", 502}, {"v5.1.0", "html", 502}, {"5.0.0", "2.9", 200}} {
		f.app.Store(entry.app)
		f.api.Store(entry.api)
		if r := f.test(t); r.Code != entry.status {
			t.Fatal(entry, r.Code, r.Body.String())
		}
	}
	f.mode.Store(8)
	decodeResponse[downloadclient.TestResult](t, f.test(t), 200)
	if f.logouts.Load() != 7 {
		t.Fatal("not all sessions cleaned up")
	}
}

func TestDownloadClientExclusionConfigurationRaceAndCancellation(t *testing.T) {
	f := newClientFixture(t)
	f.save(t, 0, clientPassword)
	f.mode.Store(6)
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- f.test(t) }()
	select {
	case <-f.entered:
	case <-time.After(time.Second):
		t.Fatal("test did not start")
	}
	if r := f.test(t); r.Code != 409 {
		t.Fatal("overlapping test accepted", r.Code)
	}
	f.save(t, 1, "changed-password")
	close(f.release)
	select {
	case r := <-done:
		if r.Code != 409 || !strings.Contains(r.Body.String(), "download-client-changed") {
			t.Fatal(r.Code, r.Body.String())
		}
	case <-time.After(time.Second):
		t.Fatal("test did not finish")
	}
	f.mode.Store(7)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	r := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/v1/download-client/test", strings.NewReader(`{}`)).WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Piratarr-Request", "1")
	response := httptest.NewRecorder()
	start := time.Now()
	f.handler.ServeHTTP(response, r)
	if response.Code != 504 || time.Since(start) > 1500*time.Millisecond || f.logouts.Load() != 2 {
		t.Fatal("cancellation or logout failed", response.Code, time.Since(start), f.logouts.Load())
	}
	f.mode.Store(0)
	decodeResponse[downloadclient.TestResult](t, f.test(t), 200)
}

func TestDownloadClientRequiresSetupAndSavedConfiguration(t *testing.T) {
	f := newCatalogFixture(t, false)
	if r := f.request(t, "POST", "/api/v1/download-client/test", `{}`); r.Code != 409 {
		t.Fatal(r.Code)
	}
	if r := f.request(t, "PUT", "/api/v1/download-client", `{"revision":0,"url":"http://localhost:8081","username":"admin","password":"dummy"}`); r.Code != 409 {
		t.Fatal(r.Code)
	}
	f = newCatalogFixture(t, true)
	if r := f.request(t, "POST", "/api/v1/download-client/test", `{}`); r.Code != 503 || !strings.Contains(r.Body.String(), "download-client-unconfigured") {
		t.Fatal(r.Code, r.Body.String())
	}
}
