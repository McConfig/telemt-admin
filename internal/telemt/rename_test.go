package telemt

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
)

func TestRenamePreservesKey(t *testing.T) {
	var mu sync.Mutex
	state := map[string]string{
		"alice": "tg://proxy?server=proxy.example.com&port=443&secret=ee00112233445566778899aabbccddeeff6578616d706c652e636f6d",
	}
	var created map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/users/alice":
			_, _ = w.Write([]byte(`{"ok":true,"data":{"username":"alice","enabled":true,"links":{"tls":["` + state["alice"] + `"]}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/users":
			_ = decodeJSON(r, &created)
			key := created["secret"].(string)
			state["bob"] = "tg://proxy?server=proxy.example.com&port=443&secret=ee" + key + "6578616d706c652e636f6d"
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"ok":true,"data":{"user":{"username":"bob","enabled":true}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/users/bob":
			_, _ = w.Write([]byte(`{"ok":true,"data":{"username":"bob","enabled":true,"links":{"tls":["` + state["bob"] + `"]}}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/users/alice":
			delete(state, "alice")
			_, _ = w.Write([]byte(`{"ok":true,"data":{"deleted":true}}`))
		default:
			http.NotFound(w, r)
		}
	})
	if err := c.Rename(context.Background(), "alice", "bob"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if created["secret"] != "00112233445566778899aabbccddeeff" {
		t.Fatalf("секрет не сохранён: %v", created)
	}
	if _, ok := state["alice"]; ok {
		t.Fatal("старое имя не удалено")
	}
	if _, ok := state["bob"]; !ok {
		t.Fatal("новое имя не создано")
	}
}

func TestRenameRollsBackOnKeyMismatch(t *testing.T) {
	deletedNew := false
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/users/alice" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"ok":true,"data":{"username":"alice","links":{"tls":["tg://proxy?secret=ee00112233445566778899aabbccddeeff6578616d706c652e636f6d"]}}}`))
		case r.URL.Path == "/v1/users" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"ok":true,"data":{"user":{"username":"bob"}}}`))
		case r.URL.Path == "/v1/users/bob" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"ok":true,"data":{"username":"bob","links":{"tls":["tg://proxy?secret=eeffeeddccbbaa998877665544332211006578616d706c652e636f6d"]}}}`))
		case r.URL.Path == "/v1/users/bob" && r.Method == http.MethodDelete:
			deletedNew = true
			_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
		default:
			http.NotFound(w, r)
		}
	})
	err := c.Rename(context.Background(), "alice", "bob")
	if err == nil {
		t.Fatal("ожидал ошибку несовпадения ключа")
	}
	if !deletedNew {
		t.Fatal("новый пользователь не удалён при откате")
	}
}

func TestRenameReportsPartialWhenDeleteFails(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/users/alice" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"ok":true,"data":{"username":"alice","links":{"tls":["tg://proxy?secret=ee00112233445566778899aabbccddeeff6578616d706c652e636f6d"]}}}`))
		case r.URL.Path == "/v1/users" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"ok":true,"data":{"user":{"username":"bob"}}}`))
		case r.URL.Path == "/v1/users/bob" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"ok":true,"data":{"username":"bob","links":{"tls":["tg://proxy?secret=ee00112233445566778899aabbccddeeff6578616d706c652e636f6d"]}}}`))
		case r.URL.Path == "/v1/users/alice" && r.Method == http.MethodDelete:
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	})
	err := c.Rename(context.Background(), "alice", "bob")
	var partial *RenamePartialError
	if !errors.As(err, &partial) || partial.Old != "alice" || partial.New != "bob" {
		t.Fatalf("ожидал RenamePartialError, получил %v", err)
	}
}
