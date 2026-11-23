package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	st, mr := newTestStore(t)

	_ = st.CreateUser(ctx, User{Name: "alice", Hash: "x", Role: RoleAdmin, CreatedAt: time.Now()})

	tok, err := st.CreateSession(ctx, "alice", "192.0.2.10", "test-agent", time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	sess, err := st.GetSession(ctx, tok)
	if err != nil || sess.User != "alice" || sess.CSRF == "" {
		t.Fatalf("GetSession = %+v, %v", sess, err)
	}

	if _, err := st.GetSession(ctx, "нет-такого"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("неизвестный токен: %v", err)
	}

	if err := st.DeleteSession(ctx, tok); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := st.GetSession(ctx, tok); !errors.Is(err, ErrNotFound) {
		t.Fatal("сессия не удалилась")
	}

	tok1, _ := st.CreateSession(ctx, "alice", "192.0.2.10", "a", time.Hour)
	tok2, _ := st.CreateSession(ctx, "alice", "192.0.2.11", "b", time.Hour)
	if err := st.DeleteUserSessions(ctx, "alice", tok1); err != nil {
		t.Fatalf("DeleteUserSessions: %v", err)
	}
	if _, err := st.GetSession(ctx, tok1); err != nil {
		t.Fatalf("текущая сессия должна остаться: %v", err)
	}
	if _, err := st.GetSession(ctx, tok2); !errors.Is(err, ErrNotFound) {
		t.Fatal("другая сессия должна быть удалена")
	}

	mr.FastForward(2 * time.Hour)
	if _, err := st.GetSession(ctx, tok1); !errors.Is(err, ErrNotFound) {
		t.Fatal("истёкшая сессия должна исчезнуть")
	}
}
