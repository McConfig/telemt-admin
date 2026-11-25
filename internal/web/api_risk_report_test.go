package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/store"
)

func riskReportServer(t *testing.T) *Server {
	t.Helper()
	s := riskServer(t)
	s.cfg.RiskIPsWarn = 0
	ctx := context.Background()
	now := time.Now().UTC()
	geo := map[string]store.IPGeo{
		"192.0.2.10":   {Country: "RU", CountryName: "Россия", ASN: 25513, ASNOrg: "MGTS"},
		"198.51.100.7": {Country: "NL", CountryName: "Нидерланды", ASN: 60781, ASNOrg: "LeaseWeb"},
	}
	if err := s.store.RecordIPObservation(ctx, "main", []string{"192.0.2.10"}, geo, 1024, now.AddDate(0, 0, -2)); err != nil {
		t.Fatal(err)
	}
	if err := s.store.RecordIPObservation(ctx, "main", []string{"192.0.2.10", "198.51.100.7"}, geo, 2048, now); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRiskReport(t *testing.T) {
	s := riskReportServer(t)
	tok, _ := seedSession(t, s, store.RoleAdmin)

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/users/main/risk-report?days=7", "", tok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}
	var report apiRiskReport
	if err := json.Unmarshal(rr.Body.Bytes(), &report); err != nil {
		t.Fatalf("разбор: %v", err)
	}
	if report.User != "main" || report.Level != 1 {
		t.Fatalf("отчёт: %+v", report)
	}
	if report.Current.ActiveIPs != 1 || report.Current.Current != 1 {
		t.Fatalf("текущие адреса: %+v", report.Current)
	}
	if report.Thresholds.WarnIPs != 0 || report.Thresholds.HighIPs != 5 {
		t.Fatalf("пороги: %+v", report.Thresholds)
	}
	if len(report.Reasons) != 2 {
		t.Fatalf("причины: %+v", report.Reasons)
	}
	codes := map[string]bool{}
	for _, reason := range report.Reasons {
		codes[reason.Code] = true
	}
	if !codes["current_ips"] || !codes["new_country"] {
		t.Fatalf("ожидались current_ips и new_country: %+v", codes)
	}
	if len(report.Days) == 0 || len(report.Days[0].IPs) != 2 {
		t.Fatalf("дни: %+v", report.Days)
	}
	for _, ip := range report.Days[0].IPs {
		if ip.DayFirst == "" || ip.DayLast == "" {
			t.Fatalf("нет времени внутри дня: %+v", ip)
		}
	}
	if report.Days[0].NewIPs != 1 || len(report.Days[0].NewCountries) != 1 {
		t.Fatalf("новые адреса и страны: %+v", report.Days[0])
	}
	if report.Days[0].NewCountries[0].Code != "NL" {
		t.Fatalf("новая страна: %+v", report.Days[0].NewCountries)
	}
}

func TestRiskReportTrustedFilters(t *testing.T) {
	s := riskReportServer(t)
	tok, _ := seedSession(t, s, store.RoleAdmin)
	if err := s.store.SetTrusted(context.Background(), "main", store.Trusted{
		Countries: []string{"NL"},
		IPs:       []string{"198.51.100.7"},
	}); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/users/main/risk-report", "", tok, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}
	var report apiRiskReport
	if err := json.Unmarshal(rr.Body.Bytes(), &report); err != nil {
		t.Fatalf("разбор: %v", err)
	}
	if report.Current.NewCountries != 0 {
		t.Fatalf("доверенная страна осталась новой: %+v", report.Current)
	}
	for _, reason := range report.Reasons {
		if reason.Code == "new_country" {
			t.Fatalf("доверенная страна попала в причины: %+v", reason)
		}
	}
	foundTrusted := false
	for _, ip := range report.Days[0].IPs {
		if ip.IP == "198.51.100.7" {
			foundTrusted = true
			if !ip.Trusted {
				t.Fatalf("адрес не помечен доверенным: %+v", ip)
			}
		}
	}
	if !foundTrusted {
		t.Fatal("доверенный адрес не найден в отчёте")
	}
}

func TestRiskReportPermissionsAndCSV(t *testing.T) {
	s := riskReportServer(t)
	admin, _ := seedSession(t, s, store.RoleAdmin)
	viewer, _ := seedSession(t, s, store.RoleViewer)

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/users/main/risk-report", "", viewer, ""))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("viewer: код %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/users/bad%20name/risk-report", "", admin, ""))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("имя: код %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, apiRequest(t, "GET", "/api/users/main/risk.csv?days=7", "", admin, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("csv: код %d: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/csv") {
		t.Fatalf("content-type: %s", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "day,distinct,max") || !strings.Contains(body, "198.51.100.7") {
		t.Fatalf("csv без ожидаемых строк:\n%s", body)
	}
	if !strings.Contains(rr.Header().Get("Content-Disposition"), "risk-main.csv") {
		t.Fatalf("имя файла: %s", rr.Header().Get("Content-Disposition"))
	}
}
