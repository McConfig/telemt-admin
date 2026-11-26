package web

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/store"
)

func TestRiskReportDaysParam(t *testing.T) {
	s := riskReportServer(t)
	tok, _ := seedSession(t, s, store.RoleAdmin)
	for _, days := range []string{"abc", "0", "-5", "999"} {
		rr := call(t, s, "GET", "/api/users/main/risk-report?days="+days, "", tok, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("days=%s: %d %s", days, rr.Code, rr.Body.String())
		}
	}
}

func TestRiskReportErrorBranches(t *testing.T) {
	s := riskReportServer(t)
	tok, _ := seedSession(t, s, store.RoleAdmin)
	viewer, viewerCSRF := seedSession(t, s, store.RoleViewer)

	if rr := call(t, s, "GET", "/api/users/missing/risk-report", "", tok, ""); rr.Code == http.StatusOK {
		t.Fatalf("несуществующий пользователь: %d", rr.Code)
	}
	if rr := call(t, s, "GET", "/api/users/missing/risk.csv", "", tok, ""); rr.Code == http.StatusOK {
		t.Fatalf("CSV несуществующего пользователя: %d", rr.Code)
	}
	if rr := call(t, s, "GET", "/api/users/main/risk.csv", "", viewer, viewerCSRF); rr.Code != http.StatusForbidden {
		t.Fatalf("просмотрщик CSV: %d", rr.Code)
	}
	if rr := call(t, s, "GET", "/api/users/bad%20name/risk.csv", "", tok, ""); rr.Code != http.StatusBadRequest {
		t.Fatalf("недопустимое имя CSV: %d", rr.Code)
	}
}

func TestRiskReportTrustedAndGeo(t *testing.T) {
	s := riskReportServer(t)
	tok, _ := seedSession(t, s, store.RoleAdmin)
	ctx := context.Background()
	if err := s.store.SetTrusted(ctx, "main", store.Trusted{IPs: []string{"192.0.2.10"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.store.SaveGeoStats(ctx, "main", 1, []string{"NL"}, []string{"Нидерланды"}, []string{"LeaseWeb"}); err != nil {
		t.Fatal(err)
	}

	rr := call(t, s, "GET", "/api/users/main/risk-report", "", tok, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}
	var report apiRiskReport
	if err := json.Unmarshal(rr.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Current.TrustedActive != 1 || report.Current.Current != 0 {
		t.Fatalf("доверенные активные адреса: %+v", report.Current)
	}
	if len(report.Current.Countries) != 1 || report.Current.Countries[0].Code != "NL" {
		t.Fatalf("страны из GeoStats: %+v", report.Current.Countries)
	}
}

func TestRiskReportSeverityReasons(t *testing.T) {
	s := riskReportServer(t)
	tok, _ := seedSession(t, s, store.RoleAdmin)
	ctx := context.Background()
	zero := 0
	if err := s.store.SetRisk(ctx, "main", store.Risk{
		WarnIPs: &zero, HighIPs: &zero, Distinct7d: &zero,
	}); err != nil {
		t.Fatal(err)
	}

	rr := call(t, s, "GET", "/api/users/main/risk-report", "", tok, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}
	var report apiRiskReport
	if err := json.Unmarshal(rr.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	severities := map[string]int{}
	for _, reason := range report.Reasons {
		severities[reason.Code] = reason.Severity
	}
	if severities["current_ips"] != 2 {
		t.Fatalf("ожидалась высокая severity: %+v", report.Reasons)
	}
	if _, ok := severities["distinct_7d"]; !ok {
		t.Fatalf("ожидалась причина distinct_7d: %+v", report.Reasons)
	}
}

func TestRiskCSVEdgeDays(t *testing.T) {
	s := testServerWithTelemt(t)
	tok, _ := seedSession(t, s, store.RoleAdmin)
	ctx := context.Background()
	now := time.Now().UTC()
	s.now = func() time.Time { return now }

	// День только с трафиком: IP-наблюдений нет, но строка дня должна попасть в CSV.
	for _, total := range []uint64{0, 500} {
		if err := s.store.RecordIPObservation(ctx, "main", nil, nil, total, now.AddDate(0, 0, -1)); err != nil {
			t.Fatal(err)
		}
	}
	// Две новые страны за сутки для основного пользователя.
	yesterday := now.AddDate(0, 0, -2)
	if err := s.store.RecordIPObservation(ctx, "main", []string{"192.0.2.20"},
		map[string]store.IPGeo{"192.0.2.20": {Country: "RU", CountryName: "Россия"}}, 100, yesterday); err != nil {
		t.Fatal(err)
	}
	geo := map[string]store.IPGeo{
		"198.51.100.20": {Country: "NL", CountryName: "Нидерланды", ASN: 1},
		"203.0.113.20":  {Country: "DE", CountryName: "Германия", ASN: 2},
	}
	if err := s.store.RecordIPObservation(ctx, "main",
		[]string{"198.51.100.20", "203.0.113.20"}, geo, 200, now); err != nil {
		t.Fatal(err)
	}

	rr := call(t, s, "GET", "/api/users/main/risk.csv?days=2", "", tok, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("CSV: %d %s", rr.Code, rr.Body.String())
	}
	reader := csv.NewReader(strings.NewReader(rr.Body.String()))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	emptyDay := false
	for _, row := range rows[1:] {
		if len(row) == 6 {
			emptyDay = true
		}
	}
	if !emptyDay {
		t.Fatalf("ожидалась строка дня без IP: %+v", rows)
	}

	if body := rr.Body.String(); !strings.Contains(body, "DE NL") && !strings.Contains(body, "NL DE") {
		t.Fatalf("в CSV нет списка новых стран: %s", body)
	}
}

func TestRiskHelperFunctions(t *testing.T) {
	if got := formatRiskTime(time.Time{}); got != "" {
		t.Fatalf("нулевое время: %q", got)
	}
	if got := formatRiskTime(time.Unix(0, 0).UTC()); got == "" {
		t.Fatal("время не отформатировано")
	}
	if got := geoipCountryName("ZZ", nil); got == "" {
		t.Fatal("нет резервного названия страны")
	}
	if got := geoipCountryName("NL", []store.RiskDayIP{{Country: "NL", CountryName: "Нидерланды"}}); got != "Нидерланды" {
		t.Fatalf("название страны: %q", got)
	}
	if got := countryList([]string{"NL", "DE", "FR"}); got != "NL, DE, FR" {
		t.Fatalf("список стран: %q", got)
	}
}
