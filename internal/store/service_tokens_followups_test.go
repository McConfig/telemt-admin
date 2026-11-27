package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/auth"
)

func TestCountServiceTokensMany(t *testing.T) {
	st, _ := newTestStore(t)
	ctx := context.Background()
	for _, name := range []string{"a", "b"} {
		if err := st.CreateService(ctx, ServiceAccount{Name: name, Scope: ScopeRead}); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := st.CreateServiceToken(ctx, "a", "", "test", nil); err != nil {
		t.Fatal(err)
	}
	counts, err := st.CountServiceTokensMany(ctx, []string{"a", "b"})
	if err != nil || counts["a"] != 1 || counts["b"] != 0 {
		t.Fatalf("%+v %v", counts, err)
	}
}

func TestExpiredTokenWithExistingKeyIsRejected(t *testing.T) {
	st, _ := newTestStore(t)
	ctx := context.Background()
	if err := st.CreateService(ctx, ServiceAccount{Name: "svc", Scope: ScopeRead}); err != nil {
		t.Fatal(err)
	}
	full, tok, err := st.CreateServiceToken(ctx, "svc", "", "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	if err := st.rdb.HSet(ctx, tokenKey(tok.ID), "expires_at", past).Err(); err != nil {
		t.Fatal(err)
	}
	if err := st.rdb.Persist(ctx, tokenKey(tok.ID)).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AuthenticateServiceToken(ctx, full); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("ожидался ErrInvalidToken: %v", err)
	}
}

func TestRawSecretNotInTokenHash(t *testing.T) {
	st, _ := newTestStore(t)
	ctx := context.Background()
	if err := st.CreateService(ctx, ServiceAccount{Name: "svc", Scope: ScopeRead}); err != nil {
		t.Fatal(err)
	}
	full, tok, err := st.CreateServiceToken(ctx, "svc", "", "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	id, secret, ok := auth.ParseServiceToken(full)
	if !ok || id != tok.ID {
		t.Fatalf("токен не разобран: %q", full)
	}
	fields, err := st.rdb.HGetAll(ctx, tokenKey(tok.ID)).Result()
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range fields {
		if value == secret || value == full || strings.Contains(value, secret) {
			t.Fatalf("сырой секрет в поле %s", name)
		}
	}
}
