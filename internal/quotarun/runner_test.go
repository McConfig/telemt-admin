package quotarun

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/settings"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
	"github.com/alicebob/miniredis/v2"
)

func TestTickResetsWhenDue(t *testing.T) {
	var resets atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/users":
			_, _ = w.Write([]byte(`{"ok":true,"data":[{"username":"alice","enabled":true,"data_quota_bytes":1000,"total_octets":900}]}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reset-quota"):
			resets.Add(1)
			_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
		default:
			http.Error(w, "no", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	mr := miniredis.RunT(t)
	st, err := store.Open(context.Background(), "redis://"+mr.Addr()+"/0", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	base, _ := url.Parse("https://tgadmin.example.com")
	cfg := &config.Config{
		BaseURL: base, HomeCountry: "RU", LogLevel: "info",
		QuotaResetEnabled: true, QuotaResetDays: 1, QuotaResetTime: "04:00",
	}
	svc := settings.New(cfg, st)
	_ = svc.Load(context.Background())
	runner := New(st, telemt.New(srv.URL, "", time.Second), svc)
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	runner.now = func() time.Time { return now }

	// Первый проход только задаёт точку отсчёта.
	runner.Tick(context.Background())
	if resets.Load() != 0 {
		t.Fatalf("первый проход сбросил квоту")
	}
	last, ok, _ := st.QuotaLastRun(context.Background(), "alice")
	if !ok || !last.Equal(now) {
		t.Fatalf("last: %s %v", last, ok)
	}

	// Сдвигаем last в прошлое, чтобы расписание стало просроченным.
	if err := st.SetQuotaLastRun(context.Background(), "alice", now.AddDate(0, 0, -2)); err != nil {
		t.Fatal(err)
	}
	runner.Tick(context.Background())
	if resets.Load() != 1 {
		t.Fatalf("сбросов %d, ожидался 1", resets.Load())
	}
	events, _ := st.RecentAudit(context.Background(), 5)
	if len(events) == 0 || events[0].Action != "quota.reset.scheduled" || events[0].Target != "alice" {
		t.Fatalf("аудит: %+v", events)
	}
}

func TestPerUserOffOverridesGlobal(t *testing.T) {
	var resets atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/users" {
			_, _ = w.Write([]byte(`{"ok":true,"data":[{"username":"alice","enabled":true}]}`))
			return
		}
		resets.Add(1)
		_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
	}))
	defer srv.Close()
	mr := miniredis.RunT(t)
	st, _ := store.Open(context.Background(), "redis://"+mr.Addr()+"/0", "")
	t.Cleanup(func() { _ = st.Close() })
	base, _ := url.Parse("https://tgadmin.example.com")
	svc := settings.New(&config.Config{
		BaseURL: base, HomeCountry: "RU", LogLevel: "info",
		QuotaResetEnabled: true, QuotaResetDays: 1, QuotaResetTime: "04:00",
	}, st)
	_ = svc.Load(context.Background())
	if err := st.SetQuotaReset(context.Background(), "alice", store.QuotaReset{Mode: "off"}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	runner := New(st, telemt.New(srv.URL, "", time.Second), svc)
	runner.now = func() time.Time { return now }
	_ = st.SetQuotaLastRun(context.Background(), "alice", now.AddDate(0, 0, -2))
	runner.Tick(context.Background())
	if resets.Load() != 0 {
		t.Fatal("off-режим не должен сбрасывать")
	}
}

func TestEffectiveQuotaReset(t *testing.T) {
	if en, _, _ := settings.EffectiveQuotaReset(true, 30, "04:00", store.QuotaReset{Mode: "off"}); en {
		t.Fatal("off")
	}
	if en, d, at := settings.EffectiveQuotaReset(true, 30, "04:00", store.QuotaReset{Mode: "custom", Days: 7, Time: "05:00"}); !en || d != 7 || at != "05:00" {
		t.Fatal("custom")
	}
	last := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	next := settings.NextQuotaReset(last, 7, "04:30")
	if next.Day() != 8 || next.Hour() != 4 || next.Minute() != 30 {
		t.Fatalf("next: %s", next)
	}
	if !settings.QuotaResetDue(last, 7, "04:30", next) {
		t.Fatal("должно быть просрочено")
	}
	if settings.QuotaResetDue(last, 7, "04:30", next.Add(-time.Minute)) {
		t.Fatal("рано")
	}
}
