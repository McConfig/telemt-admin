package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/telemt"
)

func testServerWithRename(t *testing.T) *Server {
	t.Helper()
	links := map[string]string{
		"alice": "tg://proxy?server=proxy.example.com&port=443&secret=ee00112233445566778899aabbccddeeff6578616d706c652e636f6d",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/users/alice":
			_, _ = w.Write([]byte(`{"ok":true,"data":{"username":"alice","enabled":true,"links":{"tls":["` + links["alice"] + `"]}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/users":
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			links["bob"] = "tg://proxy?server=proxy.example.com&port=443&secret=ee" + in["secret"].(string) + "6578616d706c652e636f6d"
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"ok":true,"data":{"user":{"username":"bob"}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/users/bob":
			_, _ = w.Write([]byte(`{"ok":true,"data":{"username":"bob","enabled":true,"links":{"tls":["` + links["bob"] + `"]}}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/users/alice":
			_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	s := testServer(t)
	s.telemt = telemt.New(srv.URL, "", 5*time.Second)
	return s
}

func TestRenameRedirectsToNewName(t *testing.T) {
	s := testServerWithRename(t)
	tok, csrf := adminAuth(t, s)
	form := url.Values{"csrf": {csrf}, "new_name": {"bob"}}
	req := httptest.NewRequest("POST", "/users/alice/rename", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.0.2.10:5555"
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/users/bob" {
		t.Fatalf("код %d, location %q: %s", rr.Code, rr.Header().Get("Location"), rr.Body.String())
	}
}
