package telemt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMutationsAndReady(t *testing.T) {
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/capabilities":
			_, _ = w.Write([]byte(`{"ok":true,"data":{"version":"test"}}`))
		case "/v1/users":
			_, _ = w.Write([]byte(`{"ok":true,"data":{"user":{"username":"alice","enabled":true}}}`))
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	c := New(srv.URL, "", time.Second)
	ctx := context.Background()

	if err := c.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.PatchUser(ctx, "alice", map[string]any{"max_unique_ips": 2}); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteUser(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if err := c.ResetQuota(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if err := c.RotateSecret(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if err := c.EnableUser(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if err := c.DisableUser(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	u, err := c.CreateUser(ctx, CreateInput{Username: "alice"})
	if err != nil || u == nil || u.Username != "alice" {
		t.Fatalf("создание: %+v %v", u, err)
	}
	if len(calls) < 7 {
		t.Fatalf("вызовы: %v", calls)
	}
}

func TestErrorsAndRedaction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"bad","message":"secret=00112233445566778899aabbccddeeff"}}`))
	}))
	defer srv.Close()
	err := New(srv.URL, "", time.Second).Ready(context.Background())
	if err == nil || strings.Contains(err.Error(), "00112233445566778899aabbccddeeff") {
		t.Fatalf("ошибка: %v", err)
	}
	apiErr := &APIError{Status: 500, Message: "x"}
	if !strings.Contains(apiErr.Error(), "500") {
		t.Fatalf("текст: %s", apiErr.Error())
	}

	partial := &RenamePartialError{Old: "a", New: "b", Err: context.Canceled}
	if !strings.Contains(partial.Error(), "b") || partial.Unwrap() == nil {
		t.Fatalf("partial: %v", partial)
	}
}

func TestCreateUserWithoutUserField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true,"data":{"username":"alice","enabled":true}}`))
		}
	}))
	defer srv.Close()
	c := New(srv.URL, "", time.Second)
	u, err := c.CreateUser(context.Background(), CreateInput{Username: "alice"})
	if err != nil || u.Username != "alice" {
		t.Fatalf("fallback GetUser: %+v %v", u, err)
	}
}

func TestReadyMalformedResponses(t *testing.T) {
	cases := []string{
		`garbage`,
		`{"ok":true}`,
		`{"ok":true,"data":{"x":1}}`,
	}
	for _, body := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		err := New(srv.URL, "", time.Second).Ready(context.Background())
		srv.Close()
		if body == `{"ok":true,"data":{"x":1}}` && err != nil {
			t.Errorf("валидный ответ: %v", err)
		}
	}
	if err := New("://bad", "", time.Second).Ready(context.Background()); err == nil {
		t.Error("некорректный URL не дал ошибку")
	}
}
