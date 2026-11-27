package store

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func mustService(t *testing.T, st *Store, name string, scope ServiceScope) {
	t.Helper()
	if err := st.CreateService(context.Background(), ServiceAccount{
		Name: name, Scope: scope, CreatedBy: "admin",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCreateAndAuthenticateToken(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	mustService(t, st, "monitoring", ScopeRead)

	full, tok, err := st.CreateServiceToken(ctx, "monitoring", "ci", "admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(full, "tgadm_") || tok.ID == "" {
		t.Fatalf("токен: %q %+v", full, tok)
	}
	auth, err := st.AuthenticateServiceToken(ctx, full)
	if err != nil || auth.Service.Name != "monitoring" || auth.TokenID != tok.ID {
		t.Fatalf("auth: %+v %v", auth, err)
	}
	if _, err := st.AuthenticateServiceToken(ctx, full+"x"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("чужой секрет: %v", err)
	}
	if _, err := st.AuthenticateServiceToken(ctx, "bad"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("формат: %v", err)
	}
	list, err := st.ListServiceTokens(ctx, "monitoring")
	if err != nil || len(list) != 1 || list[0].ID != tok.ID {
		t.Fatalf("list: %+v %v", list, err)
	}
	if err := st.DeleteServiceToken(ctx, "monitoring", tok.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AuthenticateServiceToken(ctx, full); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("после отзыва: %v", err)
	}
}

func TestTokenExpiryAndDisable(t *testing.T) {
	ctx := context.Background()
	st, mr := newTestStore(t)
	mustService(t, st, "svc", ScopeRead)

	exp := time.Now().Add(time.Hour)
	full, _, err := st.CreateServiceToken(ctx, "svc", "", "admin", &exp)
	if err != nil {
		t.Fatal(err)
	}
	mr.FastForward(2 * time.Hour)
	if _, err := st.AuthenticateServiceToken(ctx, full); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("истёкший: %v", err)
	}

	full2, _, err := st.CreateServiceToken(ctx, "svc", "", "admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	disabled := true
	if _, err := st.UpdateService(ctx, "svc", ServiceUpdate{Disabled: &disabled}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AuthenticateServiceToken(ctx, full2); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("отключён: %v", err)
	}
}

func TestTokenLimitLastUsedAndCascade(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	mustService(t, st, "svc", ScopeRead)

	for i := 0; i < MaxServiceTokens; i++ {
		if _, _, err := st.CreateServiceToken(ctx, "svc", "", "admin", nil); err != nil {
			t.Fatalf("токен %d: %v", i, err)
		}
	}
	if _, _, err := st.CreateServiceToken(ctx, "svc", "", "admin", nil); !errors.Is(err, ErrLimit) {
		t.Fatalf("лимит: %v", err)
	}

	mustService(t, st, "svc2", ScopeRead)
	full, tok, err := st.CreateServiceToken(ctx, "svc2", "", "admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339)
	if err := st.rdb.HSet(ctx, tokenKey(tok.ID), "last_used_at", old).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AuthenticateServiceToken(ctx, full); err != nil {
		t.Fatal(err)
	}
	first, _ := st.rdb.HGet(ctx, tokenKey(tok.ID), "last_used_at").Result()
	if first == old {
		t.Fatal("last_used_at не обновился")
	}
	if _, err := st.AuthenticateServiceToken(ctx, full); err != nil {
		t.Fatal(err)
	}
	second, _ := st.rdb.HGet(ctx, tokenKey(tok.ID), "last_used_at").Result()
	if second != first {
		t.Fatal("last_used_at обновился повторно")
	}

	if err := st.DeleteService(ctx, "svc2"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AuthenticateServiceToken(ctx, full); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("после удаления сервиса: %v", err)
	}
}

func TestTokenStoresOnlyHash(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	mustService(t, st, "svc", ScopeRead)
	full, tok, err := st.CreateServiceToken(ctx, "svc", "", "admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := st.rdb.HGetAll(ctx, tokenKey(tok.ID)).Result()
	if err != nil {
		t.Fatal(err)
	}
	if m["secret_hash"] == "" || strings.Contains(full, m["secret_hash"]) {
		t.Fatal("секрет хранится некорректно")
	}
}

func TestTokenOrphanIndexRejected(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	mustService(t, st, "svc", ScopeRead)
	full, tok, err := st.CreateServiceToken(ctx, "svc", "", "admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.rdb.SRem(ctx, serviceTokensKey("svc"), tok.ID).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AuthenticateServiceToken(ctx, full); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("осиротевший токен: %v", err)
	}
}

func TestCreateTokenConcurrentLimit(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	mustService(t, st, "svc", ScopeRead)
	var wg sync.WaitGroup
	for i := 0; i < MaxServiceTokens+10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = st.CreateServiceToken(ctx, "svc", "", "admin", nil)
		}()
	}
	wg.Wait()
	list, err := st.ListServiceTokens(ctx, "svc")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) > MaxServiceTokens {
		t.Fatalf("токенов %d, лимит %d", len(list), MaxServiceTokens)
	}
}
