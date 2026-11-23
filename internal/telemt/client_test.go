package telemt

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const usersJSON = `{"ok":true,"data":[
 {"username":"alice","enabled":true,"in_runtime":true,
  "current_connections":2,"active_unique_ips":1,
  "active_unique_ips_list":["192.0.2.10"],"total_octets":1234,
  "links":{"classic":[],"secure":[],"tls":["tg://proxy?server=proxy.example.com&port=443&secret=ee00112233445566778899aabbccddeeff6578616d706c652e636f6d"]}},
 {"username":"bob","enabled":false,"in_runtime":false,"links":null}
]}`

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(srv.URL, "", 5*time.Second)
}

func TestListUsers(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/users" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(usersJSON))
	})
	users, err := c.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 || users[0].Username != "alice" || !users[1].Disabled() {
		t.Fatalf("получено %+v", users)
	}
	if users[0].Link("tls") == "" || users[0].CurrentConnections != 2 {
		t.Fatalf("поля не разобрались: %+v", users[0])
	}
}

func TestAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"type":"not_found","message":"нет такого"}}`))
	})
	_, err := c.GetUser(context.Background(), "ghost")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 404 || apiErr.Message != "нет такого" {
		t.Fatalf("ошибка = %v", err)
	}
}
