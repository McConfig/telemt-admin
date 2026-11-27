package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

func TestPasskeyStore(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	p := Passkey{ID: "0123456789abcdef", User: "admin", Name: "MacBook",
		Credential: webauthn.Credential{ID: []byte{1, 2, 3}}}
	if err := st.CreatePasskey(ctx, p); err != nil {
		t.Fatal(err)
	}
	if n, _ := st.CountPasskeys(ctx, "admin"); n != 1 {
		t.Fatalf("count=%d", n)
	}
	list, err := st.ListPasskeys(ctx, "admin")
	if err != nil || len(list) != 1 || list[0].Name != "MacBook" {
		t.Fatalf("%+v %v", list, err)
	}
	if err := st.UpdatePasskeyUsage(ctx, p.ID, 7, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetPasskey(ctx, p.ID)
	if err != nil || got.SignCount != 7 || got.LastUsedAt.IsZero() {
		t.Fatalf("%+v %v", got, err)
	}
	if err := st.DeletePasskey(ctx, "admin", p.ID); err != nil {
		t.Fatal(err)
	}
	if n, _ := st.CountPasskeys(ctx, "admin"); n != 0 {
		t.Fatalf("count=%d", n)
	}
}

func TestPasskeyLimit(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	for i := 0; i < MaxPasskeys; i++ {
		if err := st.CreatePasskey(ctx, Passkey{ID: fmt.Sprintf("%016x", i), User: "u"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.CreatePasskey(ctx, Passkey{ID: "ffffffffffffffff", User: "u"}); !errors.Is(err, ErrLimit) {
		t.Fatalf("лимит: %v", err)
	}
}

func TestDeleteAllPasskeys(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	for i := 0; i < 3; i++ {
		_ = st.CreatePasskey(ctx, Passkey{ID: fmt.Sprintf("%016x", i), User: "u"})
	}
	if err := st.DeleteAllPasskeys(ctx, "u"); err != nil {
		t.Fatal(err)
	}
	if n, _ := st.CountPasskeys(ctx, "u"); n != 0 {
		t.Fatalf("count=%d", n)
	}
}
