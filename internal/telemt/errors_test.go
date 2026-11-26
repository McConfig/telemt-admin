package telemt

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestErrorCodes(t *testing.T) {
	cases := []struct {
		status int
		body   string
		code   string
	}{
		{status: 404, body: `{"ok":false,"error":{"code":"not_found","message":"нет"}}`, code: CodeNotFound},
		{status: 500, body: `{"ok":false,"error":{"code":"config_read_only","message":"ro"}}`, code: CodeConfigReadOnly},
		{status: 401, body: ``, code: CodeAuthFailed},
		{status: 504, body: ``, code: CodeTimeout},
		{status: 200, body: `{"ok":true}`, code: CodeInvalidResponse},
	}
	for _, c := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(c.body))
		}))
		_, err := New(srv.URL, "", time.Second).Capabilities(context.Background())
		srv.Close()
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Code != c.code {
			t.Fatalf("статус %d: err=%v, ждали %s", c.status, err, c.code)
		}
		if Hint(c.code) == "" {
			t.Fatalf("нет подсказки для %s", c.code)
		}
	}
}

func TestUnreachableCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()
	_, err := New(url, "", time.Second).Capabilities(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != CodeUnreachable {
		t.Fatalf("ошибка: %v", err)
	}
}
