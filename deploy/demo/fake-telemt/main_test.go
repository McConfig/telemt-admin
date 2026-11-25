//go:build demo

package main

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/telemt"
)

func start(t *testing.T, token string) (*httptest.Server, *telemt.Client) {
	t.Helper()
	ts := httptest.NewServer(newServer(token, "demo.example.com").routes())
	t.Cleanup(ts.Close)
	return ts, telemt.New(ts.URL, token, 5*time.Second)
}

func TestListAndGet(t *testing.T) {
	_, c := start(t, "token")
	ctx := context.Background()

	if err := c.Ready(ctx); err != nil {
		t.Fatalf("Ready: %v", err)
	}
	users, err := c.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 8 {
		t.Fatalf("пользователей %d, ожидалось 8", len(users))
	}

	u, err := c.GetUser(ctx, "router-home")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if !u.Enabled || u.TotalOctets == 0 || len(u.ActiveUniqueIPList) != 2 {
		t.Fatalf("неожиданные данные: %+v", u)
	}
	if u.MaxUniqueIPs == nil || *u.MaxUniqueIPs != 5 {
		t.Fatalf("лимит IP не совпал: %+v", u.MaxUniqueIPs)
	}
	link := u.Link("tls")
	if link == "" {
		t.Fatal("нет TLS-ссылки")
	}
	if _, err := telemt.KeyFromLink(link); err != nil {
		t.Fatalf("ссылка не разбирается: %v", err)
	}
}

func TestCreatePatchRotateDelete(t *testing.T) {
	_, c := start(t, "token")
	ctx := context.Background()

	created, err := c.CreateUser(ctx, telemt.CreateInput{Username: "new-device"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.Username != "new-device" || !created.Enabled {
		t.Fatalf("создан не тот пользователь: %+v", created)
	}

	if err := c.PatchUser(ctx, "new-device", map[string]any{
		"max_unique_ips": 4,
		"enabled":        false,
	}); err != nil {
		t.Fatalf("PatchUser: %v", err)
	}
	got, err := c.GetUser(ctx, "new-device")
	if err != nil {
		t.Fatalf("GetUser после patch: %v", err)
	}
	if got.Enabled {
		t.Fatal("пользователь должен быть выключен")
	}
	if got.MaxUniqueIPs == nil || *got.MaxUniqueIPs != 4 {
		t.Fatalf("лимит IP не применился: %+v", got.MaxUniqueIPs)
	}

	before := got.Link("tls")
	if err := c.RotateSecret(ctx, "new-device"); err != nil {
		t.Fatalf("RotateSecret: %v", err)
	}
	rotated, _ := c.GetUser(ctx, "new-device")
	if rotated.Link("tls") == before {
		t.Fatal("ссылка не изменилась после перевыпуска")
	}

	if err := c.EnableUser(ctx, "new-device"); err != nil {
		t.Fatalf("EnableUser: %v", err)
	}
	enabled, _ := c.GetUser(ctx, "new-device")
	if !enabled.Enabled {
		t.Fatal("пользователь должен быть включён")
	}

	if err := c.ResetQuota(ctx, "new-device"); err != nil {
		t.Fatalf("ResetQuota: %v", err)
	}

	if err := c.DeleteUser(ctx, "new-device"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := c.GetUser(ctx, "new-device"); err == nil {
		t.Fatal("удалённый пользователь всё ещё доступен")
	}
}

func TestAuthRequired(t *testing.T) {
	_, good := start(t, "token")
	ctx := context.Background()
	users, err := good.ListUsers(ctx)
	if err != nil {
		t.Fatalf("с токеном: %v", err)
	}
	if len(users) == 0 {
		t.Fatal("пустой список")
	}

	ts := httptest.NewServer(newServer("token", "demo.example.com").routes())
	t.Cleanup(ts.Close)
	bad := telemt.New(ts.URL, "wrong", 5*time.Second)
	if _, err := bad.ListUsers(ctx); err == nil {
		t.Fatal("запрос с неверным токеном должен падать")
	}
}
