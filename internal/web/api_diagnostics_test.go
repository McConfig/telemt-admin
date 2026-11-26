package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/collector"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/webhook"
)

func TestDiagnostics(t *testing.T) {
	s, tok, _ := coverageServer(t)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/diagnostics", "", tok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}
	var out diagnosticsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Telemt.OK || out.Telemt.Version != "test" {
		t.Fatalf("telemt: %+v", out.Telemt)
	}
	if !out.Valkey.OK {
		t.Fatalf("valkey: %+v", out.Valkey)
	}
	if out.Status == "" || out.CheckedAt == "" {
		t.Fatalf("ответ: %+v", out)
	}
	found := false
	for _, name := range out.Security.AdminsWithoutTOTP {
		if name == "user-admin" {
			found = true
		}
	}
	if !found {
		t.Fatalf("админы без 2FA: %+v", out.Security.AdminsWithoutTOTP)
	}

	viewer, viewerCSRF := seedSession(t, s, store.RoleViewer)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/diagnostics", "", viewer, viewerCSRF))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("viewer: код %d", rr.Code)
	}
}

func TestDiagnosticsWithoutTelemt(t *testing.T) {
	s := testServer(t)
	tok, _ := seedSession(t, s, store.RoleAdmin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/diagnostics", "", tok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("код %d", rr.Code)
	}
	var out diagnosticsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Telemt.OK || out.Telemt.Code != "telemt_unreachable" {
		t.Fatalf("telemt: %+v", out.Telemt)
	}
	if out.Status != "error" {
		t.Fatalf("статус: %s", out.Status)
	}
}

type diagNotifier struct {
	status webhook.Status
}

func (d diagNotifier) Emit(context.Context, string, map[string]any) {}
func (d diagNotifier) SendNow(context.Context, string, map[string]any) (int, error) {
	return 200, nil
}
func (d diagNotifier) Status() webhook.Status { return d.status }

func TestDiagnosticsWarnings(t *testing.T) {
	s, tok, _ := coverageServer(t)
	ctx := context.Background()
	if _, _, err := s.settings.Save(ctx, 0, map[string]json.RawMessage{
		"security.require_2fa":            json.RawMessage("true"),
		"security.require_2fa_grace_days": json.RawMessage("3"),
	}, nil); err != nil {
		t.Fatal(err)
	}
	s.cfg.TOTPKey = nil
	statusFile := filepath.Join(t.TempDir(), "status.json")
	if err := os.WriteFile(statusFile, []byte(`{"lastRun":"2026-11-20T04:30:00Z","result":"ok","keepDays":14,"restoreTest":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s.cfg.BackupStatus = statusFile
	s.cfg.BackupConfig = ""
	s.status = func() collector.Status {
		return collector.Status{
			Up: true, LastSuccess: time.Now().Add(-time.Hour),
			GeoIPLoaded: true, GeoIPBuilt: uint64(time.Now().Add(-40 * 24 * time.Hour).Unix()),
		}
	}
	s.SetNotifier(diagNotifier{status: webhook.Status{
		LastAt: time.Now(), LastEvent: "user.create", LastError: "500", Failed: 1,
	}})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/diagnostics", "", tok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("код %d", rr.Code)
	}
	var out diagnosticsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "error" {
		t.Fatalf("статус %s: %+v", out.Status, out.Warnings)
	}
	joined := strings.Join(out.Warnings, "|")
	for _, part := range []string{"сборщик давно", "GeoIP", "бэкап", "2FA", "вебхук"} {
		if !strings.Contains(joined, part) {
			t.Fatalf("нет предупреждения %q: %s", part, joined)
		}
	}
}
