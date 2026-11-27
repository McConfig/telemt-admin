package web

import (
	"context"
	"encoding/csv"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
	"github.com/go-chi/chi/v5"
	"github.com/skip2/go-qrcode"
)

var (
	nameRe   = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
	secretRe = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

type apiIP struct {
	IP          string `json:"ip"`
	FirstSeen   string `json:"firstSeen"`
	LastSeen    string `json:"lastSeen"`
	Count       int64  `json:"count"`
	Country     string `json:"country,omitempty"`
	CountryName string `json:"countryName,omitempty"`
	ASN         uint32 `json:"asn,omitempty"`
	ASNOrg      string `json:"asnOrg,omitempty"`
	Internal    bool   `json:"internal,omitempty"`
}

type apiUser struct {
	Username           string          `json:"username"`
	Enabled            bool            `json:"enabled"`
	InRuntime          bool            `json:"inRuntime"`
	CurrentConnections uint64          `json:"currentConnections"`
	ActiveUniqueIPs    uint64          `json:"activeUniqueIps"`
	ActiveIPs          []string        `json:"activeIps"`
	TotalOctets        uint64          `json:"totalOctets"`
	MaxUniqueIPs       *uint64         `json:"maxUniqueIps"`
	MaxTCPConns        *uint64         `json:"maxTcpConns"`
	DataQuotaBytes     *uint64         `json:"dataQuotaBytes"`
	QuotaPeriod        *string         `json:"quotaPeriod"`
	RateLimitUpBps     *uint64         `json:"rateLimitUpBps"`
	RateLimitDownBps   *uint64         `json:"rateLimitDownBps"`
	ExpirationRFC3339  *string         `json:"expiration"`
	SourceDeny         []string        `json:"sourceDeny"`
	Note               string          `json:"note,omitempty"`
	Ips24h             int64           `json:"ips24h"`
	Ips7d              int64           `json:"ips7d"`
	New24h             int64           `json:"new24h"`
	MaxToday           int64           `json:"maxToday"`
	TrafficToday       uint64          `json:"trafficToday"`
	Risk               int             `json:"risk"`
	LastSeen           string          `json:"lastSeen,omitempty"`
	IPHistory          []apiIP         `json:"ipHistory,omitempty"`
	Link               string          `json:"link,omitempty"`
	ForeignIPsCurrent  int64           `json:"foreignIpsCurrent"`
	ForeignIPs24h      int64           `json:"foreignIps24h"`
	Countries24h       int64           `json:"countries24h"`
	Countries7d        int64           `json:"countries7d"`
	NewCountries24h    int64           `json:"newCountries24h"`
	TrustedCountries   []string        `json:"trustedCountries"`
	TrustedIPs         []string        `json:"trustedIps"`
	RiskThresholds     apiRisk         `json:"riskThresholds"`
	RiskDefaults       apiRiskDefaults `json:"riskDefaults"`
	Countries          []string        `json:"countries,omitempty"`
	CountryCodes       []string        `json:"countryCodes,omitempty"`
	Operators          []string        `json:"operators,omitempty"`
}

func (s *Server) apiUserView(r *http.Request, u telemt.User) apiUser {
	out := apiUser{
		Username: u.Username, Enabled: u.Enabled, InRuntime: u.InRuntime,
		CurrentConnections: u.CurrentConnections, ActiveUniqueIPs: u.ActiveUniqueIPs,
		ActiveIPs: u.ActiveUniqueIPList, TotalOctets: u.TotalOctets,
		MaxUniqueIPs: u.MaxUniqueIPs, MaxTCPConns: u.MaxTCPConns,
		DataQuotaBytes: u.DataQuotaBytes, QuotaPeriod: u.QuotaPeriod,
		RateLimitUpBps: u.RateLimitUpBps, RateLimitDownBps: u.RateLimitDownBps,
		ExpirationRFC3339: u.ExpirationRFC3339,
	}
	if last, ok := s.store.LastSeen(r.Context(), u.Username); ok {
		out.LastSeen = last.Format(time.RFC3339)
	}
	if s.isAdmin(r) {
		if note, err := s.store.Note(r.Context(), u.Username); err == nil {
			out.Note = note
		}
	}
	if out.SourceDeny == nil {
		out.SourceDeny = []string{}
	}
	if s.isAdmin(r) {
		out.Link = u.Link(s.settings.LinkMode(r.Context()))
	}
	if geo, ok, err := s.store.GeoStats(r.Context(), u.Username); err == nil && ok {
		out.ForeignIPsCurrent = geo.ForeignCurrent
		out.Countries = geo.Countries
		out.CountryCodes = geo.CountryCodes
		out.Operators = geo.Operators
	}
	trusted, err := s.store.GetTrusted(r.Context(), u.Username)
	if err == nil {
		out.TrustedCountries = trusted.Countries
		out.TrustedIPs = trusted.IPs
	}
	riskCfg, _, err := s.store.GetRisk(r.Context(), u.Username)
	if err != nil {
		riskCfg = store.Risk{}
	}
	warn, high, distinct, newCountries := s.riskValues(r.Context(), riskCfg)
	out.RiskThresholds = riskOverrides(riskCfg)
	out.RiskDefaults = riskDefaults(warn, high, distinct, newCountries)
	stats, err := s.store.UserIPStats(r.Context(), u.Username, time.Now().UTC())
	if err == nil {
		trustedCountries := map[string]bool{}
		for _, code := range trusted.Countries {
			trustedCountries[code] = true
		}
		newCountries := int64(0)
		for _, code := range stats.NewCountryCodes {
			if !trustedCountries[code] {
				newCountries++
			}
		}
		stats.NewCountries24h = newCountries
		trustedIPs := map[string]bool{}
		for _, ip := range trusted.IPs {
			trustedIPs[ip] = true
		}
		current := int64(len(u.ActiveUniqueIPList))
		for _, ip := range u.ActiveUniqueIPList {
			if trustedIPs[ip] {
				current--
			}
		}
		out.Ips24h = stats.Distinct24h
		out.Ips7d = stats.Distinct7d
		out.New24h = stats.New24h
		out.MaxToday = stats.MaxToday
		out.TrafficToday = stats.TrafficToday
		out.ForeignIPs24h = stats.Foreign24h
		out.Countries24h = stats.Countries24h
		out.Countries7d = stats.Countries7d
		out.NewCountries24h = stats.NewCountries24h
		out.Risk = s.risk(r.Context(), current, stats, riskCfg)
	}
	return out
}

// apiIPHistory отдаёт историю адресов вместе со сведениями GeoIP.
func (s *Server) apiIPHistory(r *http.Request, name string) []apiIP {
	history, err := s.store.IPHistory(r.Context(), name)
	if err != nil {
		return nil
	}
	out := make([]apiIP, 0, len(history))
	for _, item := range history {
		out = append(out, apiIP{
			IP: item.IP, FirstSeen: item.FirstSeen.Format(time.RFC3339),
			LastSeen: item.LastSeen.Format(time.RFC3339), Count: item.Count,
			Country: item.Country, CountryName: item.CountryName,
			ASN: item.ASN, ASNOrg: item.ASNOrg, Internal: item.Internal,
		})
	}
	return out
}

func (s *Server) risk(ctx context.Context, current int64, stats store.UserIPStats, cfg store.Risk) int {
	warn, high, distinct, newCountries := s.riskValues(ctx, cfg)
	switch {
	case current > int64(high):
		return 2
	case current > int64(warn) || stats.Distinct7d > int64(distinct),
		newCountries && stats.NewCountries24h > 0:
		return 1
	default:
		return 0
	}
}

func (s *Server) apiUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.telemt.ListUsers(r.Context())
	if err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	out := make([]apiUser, 0, len(users))
	for _, u := range users {
		out = append(out, s.apiUserView(r, u))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) apiUser(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !nameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "недопустимое имя")
		return
	}
	u, err := s.telemt.GetUser(r.Context(), name)
	if err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	view := s.apiUserView(r, *u)
	if s.isAdmin(r) {
		view.IPHistory = s.apiIPHistory(r, name)
	}
	writeJSON(w, http.StatusOK, view)
}

type createUserRequest struct {
	Name              string   `json:"name"`
	Secret            string   `json:"secret"`
	MaxUniqueIPs      *uint64  `json:"maxUniqueIps"`
	MaxTCPConns       *uint64  `json:"maxTcpConns"`
	DataQuotaBytes    *uint64  `json:"dataQuotaBytes"`
	QuotaPeriod       *string  `json:"quotaPeriod"`
	RateLimitUpBps    *uint64  `json:"rateLimitUpBps"`
	RateLimitDownBps  *uint64  `json:"rateLimitDownBps"`
	ExpirationRFC3339 *string  `json:"expiration"`
	SourceDeny        []string `json:"sourceDeny"`
}

func (s *Server) apiCreateUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	var in createUserRequest
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	name := strings.TrimSpace(in.Name)
	if !nameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "имя: латиница, цифры, точка, дефис, подчёркивание, до 64 символов")
		return
	}
	secret := strings.TrimSpace(in.Secret)
	if secret != "" && !secretRe.MatchString(secret) {
		writeError(w, http.StatusBadRequest, "bad_request", "секрет: 32 hex-символа в нижнем регистре")
		return
	}
	created, err := s.telemt.CreateUser(r.Context(), telemt.CreateInput{
		Username: name, Secret: secret, MaxUniqueIPs: in.MaxUniqueIPs, MaxTCPConns: in.MaxTCPConns,
		DataQuotaBytes: in.DataQuotaBytes, QuotaPeriod: in.QuotaPeriod,
		RateLimitUpBps: in.RateLimitUpBps, RateLimitDownBps: in.RateLimitDownBps,
		ExpirationRFC3339: in.ExpirationRFC3339,
	})
	if err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	s.audit(r, "user.create", name, "ok")
	writeJSON(w, http.StatusCreated, s.apiUserView(r, *created))
}

type limitsRequest struct {
	MaxUniqueIPs      *uint64  `json:"maxUniqueIps"`
	MaxTCPConns       *uint64  `json:"maxTcpConns"`
	ClearMaxUniqueIPs bool     `json:"clearMaxUniqueIps"`
	ClearMaxTCPConns  bool     `json:"clearMaxTcpConns"`
	DataQuotaBytes    *uint64  `json:"dataQuotaBytes"`
	ClearQuota        bool     `json:"clearQuota"`
	QuotaPeriod       *string  `json:"quotaPeriod"`
	RateLimitUpBps    *uint64  `json:"rateLimitUpBps"`
	RateLimitDownBps  *uint64  `json:"rateLimitDownBps"`
	ClearRates        bool     `json:"clearRates"`
	ExpirationRFC3339 *string  `json:"expiration"`
	ClearExpiration   bool     `json:"clearExpiration"`
	SourceDeny        []string `json:"sourceDeny"`
	ClearSourceDeny   bool     `json:"clearSourceDeny"`
}

func (s *Server) apiUpdateLimits(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if !nameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "недопустимое имя")
		return
	}
	var in limitsRequest
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	patch := map[string]any{}
	if in.ClearMaxUniqueIPs {
		patch["max_unique_ips"] = nil
	} else if in.MaxUniqueIPs != nil {
		patch["max_unique_ips"] = *in.MaxUniqueIPs
	}
	if in.ClearMaxTCPConns {
		patch["max_tcp_conns"] = nil
	} else if in.MaxTCPConns != nil {
		patch["max_tcp_conns"] = *in.MaxTCPConns
	}
	if in.ClearQuota {
		patch["data_quota_bytes"] = nil
		patch["quota_period"] = nil
	} else {
		if in.DataQuotaBytes != nil {
			patch["data_quota_bytes"] = *in.DataQuotaBytes
		}
		if in.QuotaPeriod != nil {
			patch["quota_period"] = *in.QuotaPeriod
		}
	}
	if in.ClearRates {
		patch["rate_limit_up_bps"] = nil
		patch["rate_limit_down_bps"] = nil
	} else {
		if in.RateLimitUpBps != nil {
			patch["rate_limit_up_bps"] = *in.RateLimitUpBps
		}
		if in.RateLimitDownBps != nil {
			patch["rate_limit_down_bps"] = *in.RateLimitDownBps
		}
	}
	if in.ClearExpiration {
		patch["expiration_rfc3339"] = nil
	} else if in.ExpirationRFC3339 != nil {
		patch["expiration_rfc3339"] = *in.ExpirationRFC3339
	}
	if in.ClearSourceDeny {
		patch["source_deny"] = nil
	} else if len(in.SourceDeny) > 0 {
		patch["source_deny"] = in.SourceDeny
	}
	if len(patch) == 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "нет изменений")
		return
	}
	if err := s.telemt.PatchUser(r.Context(), name, patch); err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	s.audit(r, "user.limits", name, "ok")
	u, err := s.telemt.GetUser(r.Context(), name)
	if err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	view := s.apiUserView(r, *u)
	if s.isAdmin(r) {
		view.IPHistory = s.apiIPHistory(r, name)
	}
	writeJSON(w, http.StatusOK, view)
}

type confirmRequest struct {
	Confirm string `json:"confirm"`
	NewName string `json:"newName"`
}

func (s *Server) apiRotate(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	var in confirmRequest
	if err := decodeJSON(r, &in); err != nil || in.Confirm != name {
		writeError(w, http.StatusBadRequest, "bad_request", "введите имя пользователя для подтверждения")
		return
	}
	if err := s.telemt.RotateSecret(r.Context(), name); err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	s.audit(r, "user.rotate", name, "ok")
	u, err := s.telemt.GetUser(r.Context(), name)
	if err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	view := s.apiUserView(r, *u)
	if s.isAdmin(r) {
		view.IPHistory = s.apiIPHistory(r, name)
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) apiEnable(w http.ResponseWriter, r *http.Request)  { s.setEnabled(w, r, true) }
func (s *Server) apiDisable(w http.ResponseWriter, r *http.Request) { s.setEnabled(w, r, false) }

func (s *Server) setEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	var err error
	if enabled {
		err = s.telemt.EnableUser(r.Context(), name)
	} else {
		err = s.telemt.DisableUser(r.Context(), name)
	}
	if err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	action := "user.disable"
	if enabled {
		action = "user.enable"
	}
	s.audit(r, action, name, "ok")
	u, err := s.telemt.GetUser(r.Context(), name)
	if err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	view := s.apiUserView(r, *u)
	if s.isAdmin(r) {
		view.IPHistory = s.apiIPHistory(r, name)
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) apiRename(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	oldName := chi.URLParam(r, "name")
	var in confirmRequest
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	newName := strings.TrimSpace(in.NewName)
	if !nameRe.MatchString(newName) {
		writeError(w, http.StatusBadRequest, "bad_request", "новое имя: латиница, цифры, точка, дефис, подчёркивание, до 64 символов")
		return
	}
	if err := s.telemt.Rename(r.Context(), oldName, newName); err != nil {
		var partial *telemt.RenamePartialError
		if errors.As(err, &partial) {
			s.audit(r, "user.rename", oldName, "partial")
			writeError(w, http.StatusConflict, "rename_partial", "новый пользователь создан, старый удалить не удалось; оба ключа работают")
			return
		}
		s.audit(r, "user.rename", oldName, "error")
		s.telemtAPIError(w, r, err)
		return
	}
	s.audit(r, "user.rename", oldName+" -> "+newName, "ok")
	writeJSON(w, http.StatusOK, map[string]string{"username": newName})
}

func (s *Server) apiDeleteUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	var in confirmRequest
	if err := decodeJSON(r, &in); err != nil || in.Confirm != name {
		writeError(w, http.StatusBadRequest, "bad_request", "введите имя пользователя для подтверждения")
		return
	}
	users, err := s.telemt.ListUsers(r.Context())
	if err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	if len(users) <= 1 {
		writeError(w, http.StatusConflict, "last_user", "нельзя удалить последнего пользователя")
		return
	}
	if err := s.telemt.DeleteUser(r.Context(), name); err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	s.audit(r, "user.delete", name, "ok")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiQR(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	u, err := s.telemt.GetUser(r.Context(), name)
	if err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	link := u.Link(s.settings.LinkMode(r.Context()))
	if link == "" {
		writeError(w, http.StatusNotFound, "not_found", "у пользователя нет ссылки")
		return
	}
	img, err := qrcode.Encode(link, qrcode.Medium, 256)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	// #nosec G705 -- PNG сгенерирован внутри приложения, данные не пользовательские.
	_, _ = w.Write(img)
}

func (s *Server) apiResetQuota(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	locked, err := s.store.TryQuotaLock(r.Context(), name, 30*time.Second)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	if !locked {
		writeError(w, http.StatusConflict, "quota_busy", "сброс квоты уже выполняется")
		return
	}
	defer s.store.UnlockQuota(r.Context(), name)
	if err := s.telemt.ResetQuota(r.Context(), name); err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	if err := s.store.SetQuotaLastRun(r.Context(), name, time.Now().UTC()); err != nil {
		slog.Warn("не удалось сохранить время сброса квоты", "user", name, "err", err)
	}
	s.audit(r, "user.reset_quota", name, "ok")
	u, err := s.telemt.GetUser(r.Context(), name)
	if err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, s.apiUserView(r, *u))
}

func (s *Server) apiGetNote(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	note, err := s.store.Note(r.Context(), name)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"note": note})
}

func (s *Server) apiSetNote(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	var in struct {
		Note string `json:"note"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	if len(in.Note) > 200 {
		writeError(w, http.StatusBadRequest, "bad_request", "заметка не длиннее 200 символов")
		return
	}
	if err := s.store.SetNote(r.Context(), name, in.Note); err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	s.audit(r, "user.note", name, "ok")
	writeJSON(w, http.StatusOK, map[string]string{"note": in.Note})
}

func (s *Server) apiDeleteHistory(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if err := s.store.DeleteIPHistory(r.Context(), name); err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	s.audit(r, "user.history_delete", name, "ok")
	w.WriteHeader(http.StatusNoContent)
}

type apiTrafficDay struct {
	Day   string `json:"day"`
	Bytes uint64 `json:"bytes"`
}

// apiUserTraffic отдаёт трафик по дням для графика в карточке клиента.
func (s *Server) apiUserTraffic(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !nameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "недопустимое имя")
		return
	}
	days := 30
	if raw := r.URL.Query().Get("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 90 {
			writeError(w, http.StatusBadRequest, "bad_request", "days: от 1 до 90")
			return
		}
		days = n
	}
	history, err := s.store.TrafficHistory(r.Context(), name, days, s.now())
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	out := make([]apiTrafficDay, 0, len(history))
	for _, item := range history {
		out = append(out, apiTrafficDay{Day: item.Day, Bytes: item.Bytes})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) apiHistoryCSV(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	history, err := s.store.IPHistory(r.Context(), name)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"ips-"+name+".csv\"")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"ip", "first_seen", "last_seen", "observations", "country", "asn", "asn_org"})
	for _, h := range history {
		asn := ""
		if h.ASN != 0 {
			asn = strconv.FormatUint(uint64(h.ASN), 10)
		}
		_ = cw.Write([]string{
			h.IP, h.FirstSeen.Format(time.RFC3339), h.LastSeen.Format(time.RFC3339),
			strconv.FormatInt(h.Count, 10), h.Country, asn, h.ASNOrg,
		})
	}
	cw.Flush()
}
