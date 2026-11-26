package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/store"
)

func TestBulkResetQuota(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	body := `{"names":["main","other","main"]}`
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/users/reset-quota", body, tok, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}
	var out bulkResetResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Total != 3 || out.Reset != 2 || out.NotSent != 1 || len(out.Failed) != 1 {
		t.Fatalf("ответ: %+v", out)
	}
	if out.Failed[0].Status != "not_sent" || out.Failed[0].Code != "duplicate" {
		t.Fatalf("дубликат: %+v", out.Failed[0])
	}

	// Пустой список означает всех клиентов.
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/users/reset-quota", `{}`, tok, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("все: код %d", rr.Code)
	}

	viewer, viewerCSRF := seedSession(t, s, store.RoleViewer)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/users/reset-quota", `{}`, viewer, viewerCSRF))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("viewer: код %d", rr.Code)
	}
}

func TestQuotaResetSchedule(t *testing.T) {
	s := testServer(t)
	tok, csrf := seedSession(t, s, store.RoleAdmin)

	body := `{"mode":"custom","days":7,"time":"05:00"}`
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "PUT", "/api/users/main/quota-reset", body, tok, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT: код %d: %s", rr.Code, rr.Body.String())
	}
	var view quotaResetView
	if err := json.Unmarshal(rr.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.Mode != "custom" || !view.Enabled || view.UseDays != 7 || view.UseTime != "05:00" {
		t.Fatalf("view: %+v", view)
	}

	last := time.Date(2026, 11, 1, 4, 0, 0, 0, time.UTC)
	if err := s.store.SetQuotaLastRun(context.Background(), "main", last); err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/users/main/quota-reset", "", tok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET: код %d", rr.Code)
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.Last == "" || view.Next == "" {
		t.Fatalf("last/next: %+v", view)
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "PUT", "/api/users/main/quota-reset",
		`{"mode":"custom","days":0,"time":"05:00"}`, tok, csrf))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("валидация: код %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "PUT", "/api/users/main/quota-reset",
		`{"mode":"inherit"}`, tok, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("inherit: код %d", rr.Code)
	}
	if _, ok, _ := s.store.GetQuotaReset(context.Background(), "main"); ok {
		t.Fatal("переопределение должно быть удалено")
	}
}

func TestManualResetUpdatesBaselineAndLock(t *testing.T) {
	s, tok, csrf := coverageServer(t)

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/users/main/reset-quota", `{}`, tok, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("сброс: код %d: %s", rr.Code, rr.Body.String())
	}
	if _, ok, err := s.store.QuotaLastRun(context.Background(), "main"); err != nil || !ok {
		t.Fatalf("точка отсчёта не сохранена: %v %v", ok, err)
	}

	locked, err := s.store.TryQuotaLock(context.Background(), "other", 30*time.Second)
	if err != nil || !locked {
		t.Fatalf("блокировка: %v %v", locked, err)
	}
	defer s.store.UnlockQuota(context.Background(), "other")

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/users/other/reset-quota", `{}`, tok, csrf))
	if rr.Code != http.StatusConflict {
		t.Fatalf("занято: код %d: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "POST", "/api/users/reset-quota",
		`{"names":["other"]}`, tok, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("массовый: код %d", rr.Code)
	}
	var out bulkResetResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Refused != 1 || out.Reset != 0 || out.Failed[0].Code != "quota_busy" {
		t.Fatalf("ответ: %+v", out)
	}
}
