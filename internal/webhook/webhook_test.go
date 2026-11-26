package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/settings"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/alicebob/miniredis/v2"
)

func testSender(t *testing.T, handler http.HandlerFunc) (*Sender, *settings.Service, *httptest.Server) {
	t.Helper()
	target := httptest.NewServer(handler)
	t.Cleanup(target.Close)
	mr := miniredis.RunT(t)
	st, err := store.Open(context.Background(), "redis://"+mr.Addr()+"/0", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	base, _ := url.Parse("https://tgadmin.example.com")
	cfg := &config.Config{
		BaseURL: base, CollectInterval: time.Minute, SessionTTL: time.Hour,
		LinkMode: "tls", RiskIPsWarn: 3, RiskIPsHigh: 5, RiskDistinct7d: 15,
		HomeCountry: "RU", Retention: 720 * time.Hour,
		LoginMaxFailures: 10, LoginWindow: 15 * time.Minute,
		TOTPPendingTTL: 5 * time.Minute, TOTPMaxAttempts: 5,
		AuditMaxEvents: 10000, PasswordMinLength: 12, DefaultChartDays: 30,
		LogLevel: "info", BackupSchedule: "04:30", BackupKeepDays: 14,
		BackupRestoreTest: true,
	}
	svc := settings.New(cfg, st)
	if err := svc.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Save(context.Background(), 0, map[string]json.RawMessage{
		"notifications.webhook_url":    json.RawMessage(`"` + target.URL + `"`),
		"notifications.webhook_secret": json.RawMessage(`"s3cret"`),
		"notifications.events":         json.RawMessage(`"user.create,user.rotate"`),
	}, nil); err != nil {
		t.Fatal(err)
	}
	return New(svc), svc, target
}

func TestSendNowSignsAndDelivers(t *testing.T) {
	var body []byte
	var event, timestamp, signature string
	sender, _, _ := testSender(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		event = r.Header.Get("X-TeleMT-Event")
		timestamp = r.Header.Get("X-TeleMT-Timestamp")
		signature = r.Header.Get("X-TeleMT-Signature")
		w.WriteHeader(http.StatusNoContent)
	})
	status, err := sender.SendNow(context.Background(), "test", map[string]any{"message": "ok"})
	if err != nil || status != http.StatusNoContent {
		t.Fatalf("отправка: %d %v", status, err)
	}
	if event != "test" || timestamp == "" || signature == "" {
		t.Fatalf("заголовки: event=%s ts=%s sig=%s", event, timestamp, signature)
	}
	mac := hmac.New(sha256.New, []byte("s3cret"))
	_, _ = mac.Write([]byte(timestamp + "." + string(body)))
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if signature != want {
		t.Fatalf("подпись: %s, ожидалась %s", signature, want)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["event"] != "test" {
		t.Fatalf("тело: %s", body)
	}
}

func TestSendNowDisabled(t *testing.T) {
	mr := miniredis.RunT(t)
	st, _ := store.Open(context.Background(), "redis://"+mr.Addr()+"/0", "")
	t.Cleanup(func() { _ = st.Close() })
	base, _ := url.Parse("https://tgadmin.example.com")
	cfg := &config.Config{BaseURL: base, HomeCountry: "RU", LogLevel: "info"}
	svc := settings.New(cfg, st)
	_ = svc.Load(context.Background())
	sender := New(svc)
	if _, err := sender.SendNow(context.Background(), "test", nil); err == nil {
		t.Fatal("ожидалась ошибка без URL")
	}
}

func TestEventEnabled(t *testing.T) {
	if !eventEnabled([]string{"user.create", "user.rotate"}, "user.create") {
		t.Fatal("событие должно быть включено")
	}
	if eventEnabled([]string{"user.create"}, "user.delete") {
		t.Fatal("событие не должно быть включено")
	}
	if !eventEnabled([]string{"*"}, "anything") {
		t.Fatal("wildcard должен включать всё")
	}
}

func TestRunDeliversAndFilters(t *testing.T) {
	events := make(chan string, 2)
	sender, _, _ := testSender(t, func(w http.ResponseWriter, r *http.Request) {
		events <- r.Header.Get("X-TeleMT-Event")
		w.WriteHeader(http.StatusNoContent)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sender.Run(ctx)

	sender.Emit(ctx, "user.create", map[string]any{"target": "phone"})
	select {
	case event := <-events:
		if event != "user.create" {
			t.Fatalf("событие %s", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("событие не доставлено")
	}
	deadline := time.Now().Add(time.Second)
	for {
		if status := sender.Status(); status.Sent == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("статус: %+v", sender.Status())
		}
		time.Sleep(10 * time.Millisecond)
	}

	sender.Emit(ctx, "user.delete", nil)
	select {
	case event := <-events:
		t.Fatalf("отфильтрованное событие доставлено: %s", event)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestSendNowServerError(t *testing.T) {
	sender, _, _ := testSender(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	status, err := sender.SendNow(context.Background(), "test", nil)
	if err == nil || status != http.StatusInternalServerError {
		t.Fatalf("статус %d, ошибка %v", status, err)
	}
}
