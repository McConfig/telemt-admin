package web

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/McConfig/telemt-admin/internal/geoip"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/go-chi/chi/v5"
)

type apiRiskCountry struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type apiRiskReason struct {
	Code      string   `json:"code"`
	Severity  int      `json:"severity"`
	Value     int64    `json:"value"`
	Threshold int64    `json:"threshold,omitempty"`
	Countries []string `json:"countries,omitempty"`
	Message   string   `json:"message"`
}

type apiRiskIP struct {
	IP          string `json:"ip"`
	Country     string `json:"country,omitempty"`
	CountryName string `json:"countryName,omitempty"`
	ASN         uint32 `json:"asn,omitempty"`
	ASNOrg      string `json:"asnOrg,omitempty"`
	Internal    bool   `json:"internal,omitempty"`
	FirstSeen   string `json:"firstSeen,omitempty"`
	LastSeen    string `json:"lastSeen,omitempty"`
	DayFirst    string `json:"dayFirst,omitempty"`
	DayLast     string `json:"dayLast,omitempty"`
	Count       int64  `json:"count"`
	New         bool   `json:"new"`
	Trusted     bool   `json:"trusted"`
}

type apiRiskDay struct {
	Day          string           `json:"day"`
	Distinct     int64            `json:"distinct"`
	Max          int64            `json:"max"`
	NewIPs       int64            `json:"newIps"`
	NewCountries []apiRiskCountry `json:"newCountries,omitempty"`
	Traffic      uint64           `json:"traffic"`
	IPs          []apiRiskIP      `json:"ips"`
}

type apiRiskCurrent struct {
	ActiveIPs     int64            `json:"activeIps"`
	TrustedActive int64            `json:"trustedActive"`
	Current       int64            `json:"current"`
	Distinct24h   int64            `json:"distinct24h"`
	Distinct7d    int64            `json:"distinct7d"`
	MaxToday      int64            `json:"maxToday"`
	NewIPs24h     int64            `json:"newIps24h"`
	NewCountries  int64            `json:"newCountries"`
	ForeignIPs24h int64            `json:"foreignIps24h"`
	Countries     []apiRiskCountry `json:"countries,omitempty"`
}

type apiRiskReport struct {
	User       string          `json:"user"`
	Level      int             `json:"level"`
	Custom     bool            `json:"custom"`
	Thresholds apiRiskDefaults `json:"thresholds"`
	Current    apiRiskCurrent  `json:"current"`
	Reasons    []apiRiskReason `json:"reasons"`
	Days       []apiRiskDay    `json:"days"`
}

func riskDaysParam(r *http.Request) int {
	raw := r.URL.Query().Get("days")
	if raw == "" {
		return 7
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 7
	}
	if n > 30 {
		return 30
	}
	return n
}

func (s *Server) apiRiskReport(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if !nameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "недопустимое имя")
		return
	}
	report, err := s.buildRiskReport(r, name, riskDaysParam(r))
	if err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) apiRiskCSV(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if !nameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "недопустимое имя")
		return
	}
	report, err := s.buildRiskReport(r, name, riskDaysParam(r))
	if err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"risk-"+name+".csv\"")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"day", "distinct", "max", "new_ips", "new_countries", "traffic_bytes",
		"ip", "country", "country_name", "asn", "asn_org",
		"first_seen", "last_seen", "day_first", "day_last",
		"observations", "new", "trusted",
	})
	for _, day := range report.Days {
		countries := ""
		for i, c := range day.NewCountries {
			if i > 0 {
				countries += " "
			}
			countries += c.Code
		}
		if len(day.IPs) == 0 {
			_ = cw.Write([]string{
				day.Day, strconv.FormatInt(day.Distinct, 10), strconv.FormatInt(day.Max, 10),
				strconv.FormatInt(day.NewIPs, 10), countries,
				strconv.FormatUint(day.Traffic, 10),
			})
			continue
		}
		for _, ip := range day.IPs {
			asn := ""
			if ip.ASN != 0 {
				asn = strconv.FormatUint(uint64(ip.ASN), 10)
			}
			_ = cw.Write([]string{
				day.Day, strconv.FormatInt(day.Distinct, 10), strconv.FormatInt(day.Max, 10),
				strconv.FormatInt(day.NewIPs, 10), countries,
				strconv.FormatUint(day.Traffic, 10),
				ip.IP, ip.Country, ip.CountryName, asn, ip.ASNOrg,
				ip.FirstSeen, ip.LastSeen, ip.DayFirst, ip.DayLast,
				strconv.FormatInt(ip.Count, 10),
				strconv.FormatBool(ip.New), strconv.FormatBool(ip.Trusted),
			})
		}
	}
	cw.Flush()
}

func (s *Server) buildRiskReport(r *http.Request, name string, days int) (apiRiskReport, error) {
	ctx := r.Context()
	now := s.now().UTC()

	u, err := s.telemt.GetUser(ctx, name)
	if err != nil {
		return apiRiskReport{}, err
	}
	stats, err := s.store.UserIPStats(ctx, name, now)
	if err != nil {
		return apiRiskReport{}, err
	}
	trusted, err := s.store.GetTrusted(ctx, name)
	if err != nil {
		return apiRiskReport{}, err
	}
	riskCfg, _, err := s.store.GetRisk(ctx, name)
	if err != nil {
		riskCfg = store.Risk{}
	}
	warn, high, distinct, newCountriesOn := s.riskValues(ctx, riskCfg)

	trustedIPs := make(map[string]bool, len(trusted.IPs))
	for _, ip := range trusted.IPs {
		trustedIPs[ip] = true
	}
	trustedCountry := make(map[string]bool, len(trusted.Countries))
	for _, code := range trusted.Countries {
		trustedCountry[code] = true
	}

	active := int64(len(u.ActiveUniqueIPList))
	current := active
	for _, ip := range u.ActiveUniqueIPList {
		if trustedIPs[ip] {
			current--
		}
	}
	if current < 0 {
		current = 0
	}

	newCodes := make([]string, 0, len(stats.NewCountryCodes))
	for _, code := range stats.NewCountryCodes {
		if !trustedCountry[code] {
			newCodes = append(newCodes, code)
		}
	}
	stats.NewCountries24h = int64(len(newCodes))

	level := s.risk(ctx, current, stats, riskCfg)
	reasons := make([]apiRiskReason, 0, 3)
	switch {
	case current > int64(high):
		reasons = append(reasons, apiRiskReason{
			Code: "current_ips", Severity: 2, Value: current, Threshold: int64(high),
			Message: fmt.Sprintf("Активных адресов %d при высоком пороге %d", current, high),
		})
	case current > int64(warn):
		reasons = append(reasons, apiRiskReason{
			Code: "current_ips", Severity: 1, Value: current, Threshold: int64(warn),
			Message: fmt.Sprintf("Активных адресов %d при пороге предупреждения %d", current, warn),
		})
	}
	if stats.Distinct7d > int64(distinct) {
		reasons = append(reasons, apiRiskReason{
			Code: "distinct_7d", Severity: 1, Value: stats.Distinct7d, Threshold: int64(distinct),
			Message: fmt.Sprintf("Уникальных адресов за 7 дней %d при пороге %d", stats.Distinct7d, distinct),
		})
	}
	if newCountriesOn && len(newCodes) > 0 {
		reasons = append(reasons, apiRiskReason{
			Code: "new_country", Severity: 1, Value: int64(len(newCodes)),
			Countries: newCodes,
			Message:   fmt.Sprintf("Новые страны за сутки: %s", countryList(newCodes)),
		})
	}

	currentCountries := make([]apiRiskCountry, 0, 8)
	if geo, ok, err := s.store.GeoStats(ctx, name); err == nil && ok {
		for _, code := range geo.CountryCodes {
			currentCountries = append(currentCountries, apiRiskCountry{
				Code: code, Name: geoipCountryName(code, nil),
			})
		}
	}

	dayRows, err := s.store.RiskDays(ctx, name, days, now)
	if err != nil {
		return apiRiskReport{}, err
	}
	outDays := make([]apiRiskDay, 0, len(dayRows))
	for _, day := range dayRows {
		ips := make([]apiRiskIP, 0, len(day.IPs))
		for _, item := range day.IPs {
			ips = append(ips, apiRiskIP{
				IP: item.IP, Country: item.Country, CountryName: item.CountryName,
				ASN: item.ASN, ASNOrg: item.ASNOrg, Internal: item.Internal,
				FirstSeen: formatRiskTime(item.FirstSeen), LastSeen: formatRiskTime(item.LastSeen),
				DayFirst: formatRiskTime(item.DayFirst), DayLast: formatRiskTime(item.DayLast),
				Count: item.Count, New: item.New,
				Trusted: trustedIPs[item.IP] || trustedCountry[item.Country],
			})
		}
		newCountries := make([]apiRiskCountry, 0, len(day.NewCountries))
		for _, code := range day.NewCountries {
			if trustedCountry[code] {
				continue
			}
			newCountries = append(newCountries, apiRiskCountry{Code: code, Name: geoipCountryName(code, day.IPs)})
		}
		outDays = append(outDays, apiRiskDay{
			Day: day.Day, Distinct: day.Distinct, Max: day.Max, NewIPs: day.NewIPs,
			NewCountries: newCountries, Traffic: day.Traffic, IPs: ips,
		})
	}

	return apiRiskReport{
		User: name, Level: level, Custom: !riskCfg.Empty(),
		Thresholds: riskDefaults(warn, high, distinct, newCountriesOn),
		Current: apiRiskCurrent{
			ActiveIPs: active, TrustedActive: active - current, Current: current,
			Distinct24h: stats.Distinct24h, Distinct7d: stats.Distinct7d,
			MaxToday: stats.MaxToday, NewIPs24h: stats.New24h,
			NewCountries: int64(len(newCodes)), ForeignIPs24h: stats.Foreign24h,
			Countries: currentCountries,
		},
		Reasons: reasons,
		Days:    outDays,
	}, nil
}

func formatRiskTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func geoipCountryName(code string, ips []store.RiskDayIP) string {
	for _, ip := range ips {
		if ip.Country == code && ip.CountryName != "" {
			return ip.CountryName
		}
	}
	return geoip.CountryName(code, code)
}

func countryList(codes []string) string {
	out := ""
	for i, code := range codes {
		if i > 0 {
			out += ", "
		}
		out += code
	}
	return out
}
