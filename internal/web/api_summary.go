package web

import (
	"net/http"
	"strconv"
	"time"

	"github.com/McConfig/telemt-admin/internal/geoip"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/go-chi/chi/v5"
)

type apiCollectorStatus struct {
	Up          bool   `json:"up"`
	LastSuccess string `json:"lastSuccess,omitempty"`
	LastError   string `json:"lastError,omitempty"`
}

type apiGeoIPStatus struct {
	Loaded bool   `json:"loaded"`
	Built  string `json:"built,omitempty"`
}

type apiSummary struct {
	Users          int                `json:"users"`
	Enabled        int                `json:"enabled"`
	RiskHigh       int                `json:"riskHigh"`
	RiskWarn       int                `json:"riskWarn"`
	ForeignCurrent int64              `json:"foreignCurrent"`
	NewCountries   int64              `json:"newCountries"`
	TrafficToday   uint64             `json:"trafficToday"`
	Collector      apiCollectorStatus `json:"collector"`
	GeoIP          apiGeoIPStatus     `json:"geoip"`
}

func (s *Server) apiSummary(w http.ResponseWriter, r *http.Request) {
	users, err := s.telemt.ListUsers(r.Context())
	if err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	out := apiSummary{Users: len(users)}
	now := s.now().UTC()
	for _, u := range users {
		if u.Enabled {
			out.Enabled++
		}
		stats, err := s.store.UserIPStats(r.Context(), u.Username, now)
		if err != nil {
			continue
		}
		riskCfg, _, err := s.store.GetRisk(r.Context(), u.Username)
		if err != nil {
			riskCfg = store.Risk{}
		}
		switch s.risk(int64(len(u.ActiveUniqueIPList)), stats, riskCfg) {
		case 2:
			out.RiskHigh++
		case 1:
			out.RiskWarn++
		}
		out.ForeignCurrent += stats.Foreign24h
		out.NewCountries += stats.NewCountries24h
		out.TrafficToday += stats.TrafficToday
	}
	if s.status != nil {
		st := s.status()
		out.Collector = apiCollectorStatus{Up: st.Up, LastError: st.LastError}
		if !st.LastSuccess.IsZero() {
			out.Collector.LastSuccess = st.LastSuccess.Format(time.RFC3339)
		}
		out.GeoIP = apiGeoIPStatus{Loaded: st.GeoIPLoaded}
		if st.GeoIPBuilt > 0 {
			// #nosec G115 -- время сборки базы заведомо меньше int64.
			out.GeoIP.Built = time.Unix(int64(st.GeoIPBuilt), 0).UTC().Format(time.RFC3339)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

type apiTimelineCountry struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type apiTimelineDay struct {
	Day          string               `json:"day"`
	NewIPs       int                  `json:"newIps"`
	NewCountries []apiTimelineCountry `json:"newCountries,omitempty"`
	MaxIPs       int64                `json:"maxIps"`
	Bytes        uint64               `json:"bytes"`
}

func (s *Server) apiUserTimeline(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !nameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "недопустимое имя")
		return
	}
	days := 14
	if raw := r.URL.Query().Get("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 90 {
			writeError(w, http.StatusBadRequest, "bad_request", "days: от 1 до 90")
			return
		}
		days = n
	}
	items, err := s.store.Timeline(r.Context(), name, days, s.now())
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	out := make([]apiTimelineDay, 0, len(items))
	for _, item := range items {
		day := apiTimelineDay{Day: item.Day, NewIPs: item.NewIPs, MaxIPs: item.MaxIPs, Bytes: item.Bytes}
		for _, code := range item.NewCountries {
			day.NewCountries = append(day.NewCountries, apiTimelineCountry{
				Code: code, Name: geoip.CountryName(code, ""),
			})
		}
		out = append(out, day)
	}
	writeJSON(w, http.StatusOK, out)
}
