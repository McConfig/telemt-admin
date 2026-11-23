package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func newTestStore(t *testing.T) (*Store, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	st, err := Open(context.Background(), "redis://"+mr.Addr()+"/0", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st, mr
}

func TestUserLifecycle(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)

	u := User{Name: "alice", Hash: "$argon2id$test", Role: RoleAdmin, CreatedAt: time.Now().UTC()}
	if err := st.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := st.CreateUser(ctx, u); !errors.Is(err, ErrExists) {
		t.Fatalf("повторное создание: %v, хочу ErrExists", err)
	}

	got, err := st.GetUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.Role != RoleAdmin || got.Disabled {
		t.Fatalf("получено %+v", got)
	}

	if err := st.SetDisabled(ctx, "alice", true); err != nil {
		t.Fatalf("SetDisabled: %v", err)
	}
	got, _ = st.GetUser(ctx, "alice")
	if !got.Disabled {
		t.Fatal("Disabled не сохранился")
	}

	list, err := st.ListUsers(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "alice" {
		t.Fatalf("ListUsers = %v, %v", list, err)
	}

	if n, _ := st.CountUsers(ctx); n != 1 {
		t.Fatalf("CountUsers = %d", n)
	}

	if err := st.DeleteUser(ctx, "alice"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := st.GetUser(ctx, "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("после удаления: %v", err)
	}
}
