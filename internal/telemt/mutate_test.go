package telemt

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func decodeJSON(r *http.Request, out any) error {
	return json.NewDecoder(r.Body).Decode(out)
}

func TestCreateUserSendsSecret(t *testing.T) {
	var got map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/users" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true,"data":{"user":{"username":"alice","enabled":true},"secret":"00112233445566778899aabbccddeeff"}}`))
	})
	if _, err := c.CreateUser(context.Background(), CreateInput{Username: "alice"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if got["username"] != "alice" {
		t.Fatalf("тело запроса: %v", got)
	}
}

func TestKeyFromLink(t *testing.T) {
	link := "tg://proxy?server=proxy.example.com&port=443&secret=ee00112233445566778899aabbccddeeff6578616d706c652e636f6d"
	key, err := KeyFromLink(link)
	if err != nil {
		t.Fatalf("KeyFromLink: %v", err)
	}
	if key != "00112233445566778899aabbccddeeff" {
		t.Fatalf("ключ = %q", key)
	}
	if _, err := KeyFromLink("tg://proxy?secret=zz"); err == nil {
		t.Fatal("ожидал ошибку формата")
	}
}
