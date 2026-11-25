package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
)

type fakeUser struct {
	Username string        `json:"username"`
	Enabled  bool          `json:"enabled"`
	Links    *telemt.Links `json:"links"`
}

func coverageServer(t *testing.T) (*Server, string, string) {
	t.Helper()
	st, _ := newTestStoreMR(t)
	var mu sync.Mutex
	users := map[string]fakeUser{
		"main":   {Username: "main", Enabled: true, Links: &telemt.Links{TLS: []string{testLink}}},
		"other":  {Username: "other", Enabled: true, Links: &telemt.Links{TLS: []string{testLink}}},
		"nolink": {Username: "nolink", Enabled: true},
	}
	env := func(w http.ResponseWriter, data any) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": data})
	}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		p := r.URL.Path
		switch {
		case r.Method == http.MethodGet && p == "/v1/capabilities":
			env(w, map[string]any{"version": "test"})
		case r.Method == http.MethodGet && p == "/v1/users":
			list := make([]fakeUser, 0, len(users))
			for _, u := range users {
				list = append(list, u)
			}
			env(w, list)
		case r.Method == http.MethodGet && strings.HasPrefix(p, "/v1/users/"):
			name := strings.TrimPrefix(p, "/v1/users/")
			u, ok := users[name]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"not_found","message":"нет"}}`))
				return
			}
			env(w, u)
		case r.Method == http.MethodPost && p == "/v1/users":
			var in telemt.CreateInput
			_ = json.NewDecoder(r.Body).Decode(&in)
			u := fakeUser{Username: in.Username, Enabled: true, Links: &telemt.Links{TLS: []string{testLink}}}
			users[in.Username] = u
			env(w, map[string]any{"user": u})
		case r.Method == http.MethodDelete && strings.HasPrefix(p, "/v1/users/"):
			delete(users, strings.TrimPrefix(p, "/v1/users/"))
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPatch && strings.HasPrefix(p, "/v1/users/"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && (strings.HasSuffix(p, "/rotate-secret") ||
			strings.HasSuffix(p, "/enable") || strings.HasSuffix(p, "/disable") ||
			strings.HasSuffix(p, "/reset-quota")):
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(fake.Close)

	cfg := &config.Config{
		Listen: ":0", LinkMode: "tls", SessionTTL: time.Hour, CookieSecure: true,
		BaseURL:      mustURL("https://tgadmin.example.com"),
		AllowedCIDRs: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
		TOTPKey:      []byte("0123456789abcdef0123456789abcdef"),
	}
	s := New(cfg, st, telemt.New(fake.URL, "", 2*time.Second), nil)
	token, csrf := seedSession(t, s, store.RoleAdmin)
	return s, token, csrf
}

func seedCoverageData(t *testing.T, s *Server) {
	t.Helper()
	now := time.Now().UTC()
	geo := map[string]store.IPGeo{
		"192.0.2.10": {Country: "NL", CountryName: "Нидерланды", ASN: 64500, ASNOrg: "Test"},
	}
	if err := s.store.RecordIPObservation(context.Background(), "main", []string{"192.0.2.10"}, geo, 4096, now); err != nil {
		t.Fatal(err)
	}
	_ = s.store.SetNote(context.Background(), "main", "телефон")
	_ = s.store.SaveGeoStats(context.Background(), "main", 1, []string{"NL"}, []string{"Нидерланды"}, []string{"LeaseWeb"})
	_ = s.store.SetTrusted(context.Background(), "main", store.Trusted{Countries: []string{"NL"}, IPs: []string{"192.0.2.10"}})
}

func call(t *testing.T, s *Server, method, path, body, token, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, method, path, body, token, csrf))
	return rr
}

func TestAPIReadEndpoints(t *testing.T) {
	s, token, csrf := coverageServer(t)
	seedCoverageData(t, s)
	cases := []struct {
		method, path string
		want         int
	}{
		{"GET", "/api/meta", 200}, {"GET", "/api/summary", 200},
		{"GET", "/api/users", 200}, {"GET", "/api/users/main", 200},
		{"GET", "/api/users/main/traffic?days=7", 200},
		{"GET", "/api/users/main/timeline?days=7", 200},
		{"GET", "/api/users/main/trusted", 200},
		{"GET", "/api/users/main/note", 200},
		{"GET", "/api/users/main/history.csv", 200},
		{"GET", "/api/users/main/qr.png", 200},
		{"GET", "/api/users/missing/qr.png", 404},
		{"GET", "/api/users/nolink/qr.png", 404},
		{"GET", "/api/audit?q=main&limit=10", 200},
		{"GET", "/api/audit.csv", 200},
		{"GET", "/api/account/sessions", 200},
		{"GET", "/api/account/totp", 200},
		{"GET", "/api/admins", 200},
	}
	for _, c := range cases {
		if rr := call(t, s, c.method, c.path, "", token, csrf); rr.Code != c.want {
			t.Errorf("%s %s: %d %s", c.method, c.path, rr.Code, rr.Body.String())
		}
	}
}

func TestAPIUserMutations(t *testing.T) {
	s, token, csrf := coverageServer(t)
	seedCoverageData(t, s)
	cases := []struct {
		method, path, body string
		want               int
	}{
		{"POST", "/api/users", `{"name":"newclient"}`, 201},
		{"PATCH", "/api/users/main", `{"maxUniqueIps":3}`, 200},
		{"POST", "/api/users/main/rotate", `{"confirm":"main"}`, 200},
		{"POST", "/api/users/main/enable", `{}`, 200},
		{"POST", "/api/users/main/disable", `{}`, 200},
		{"POST", "/api/users/main/reset-quota", `{}`, 200},
		{"PUT", "/api/users/main/note", `{"note":"новая"}`, 200},
		{"PUT", "/api/users/main/trusted", `{"countries":["TR"],"ips":["203.0.113.10"]}`, 200},
		{"DELETE", "/api/users/main/history", `{}`, 204},
		{"POST", "/api/users/main/rename", `{"newName":"main2"}`, 200},
		{"DELETE", "/api/users/main2", `{"confirm":"main2"}`, 204},
	}
	for _, c := range cases {
		if rr := call(t, s, c.method, c.path, c.body, token, csrf); rr.Code != c.want {
			t.Errorf("%s %s: %d %s", c.method, c.path, rr.Code, rr.Body.String())
		}
	}
}

func TestAPIAdminMutations(t *testing.T) {
	s, token, csrf := coverageServer(t)
	steps := []struct {
		method, path, body string
		want               int
	}{
		{"POST", "/api/admins", `{"name":"watcher","role":"viewer","password":"long password 123"}`, 201},
		{"PATCH", "/api/admins/watcher", `{"role":"admin"}`, 200},
		{"POST", "/api/admins/watcher/password", `{"password":"another long password"}`, 200},
		{"POST", "/api/admins/watcher/sessions/revoke", `{}`, 204},
		{"DELETE", "/api/admins/watcher", `{}`, 204},
	}
	for _, c := range steps {
		if rr := call(t, s, c.method, c.path, c.body, token, csrf); rr.Code != c.want {
			t.Errorf("%s %s: %d %s", c.method, c.path, rr.Code, rr.Body.String())
		}
	}
	if err := s.store.CreateUser(context.Background(), store.User{Name: "second", Hash: "x", Role: store.RoleAdmin, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	_ = s.store.EnableTOTP(context.Background(), "second", "enc", 0, []string{"hash"})
	if rr := call(t, s, "POST", "/api/admins/second/totp-reset", `{}`, token, csrf); rr.Code != 204 {
		t.Fatalf("totp-reset: %d %s", rr.Code, rr.Body.String())
	}
}

func TestAPIViewerForbidden(t *testing.T) {
	s, _, _ := coverageServer(t)
	viewer, viewerCSRF := seedSession(t, s, store.RoleViewer)
	cases := []struct{ method, path, body string }{
		{"POST", "/api/users", `{"name":"x"}`}, {"PATCH", "/api/users/main", `{}`},
		{"DELETE", "/api/users/main", `{"confirm":"main"}`},
		{"POST", "/api/admins", `{"name":"x","role":"viewer"}`},
		{"GET", "/api/users/main/trusted", ""}, {"GET", "/api/users/main/qr.png", ""},
	}
	for _, c := range cases {
		if rr := call(t, s, c.method, c.path, c.body, viewer, viewerCSRF); rr.Code != http.StatusForbidden {
			t.Errorf("%s %s: %d", c.method, c.path, rr.Code)
		}
	}
}

func TestAPIErrors(t *testing.T) {
	s, token, csrf := coverageServer(t)
	cases := []struct {
		method, path, body string
		want               int
	}{
		{"POST", "/api/users", `{`, 400}, {"GET", "/api/users/bad name", "", 400},
		{"GET", "/api/users/missing", "", 404},
		{"POST", "/api/users/main/rotate", `{"confirm":"wrong"}`, 400},
		{"PUT", "/api/users/main/trusted", `{"countries":["X"]}`, 400},
		{"POST", "/api/login/totp", `{"code":"123456"}`, 401},
	}
	for _, c := range cases {
		if rr := call(t, s, c.method, c.path, c.body, token, csrf); rr.Code != c.want {
			t.Errorf("%s %s: %d %s", c.method, c.path, rr.Code, rr.Body.String())
		}
	}
}
