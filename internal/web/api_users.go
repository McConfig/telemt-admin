package web

import (
	"errors"
	"net/http"
	"time"

	"regexp"
	"strings"

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
	IP        string `json:"ip"`
	FirstSeen string `json:"firstSeen"`
	LastSeen  string `json:"lastSeen"`
	Count     int64  `json:"count"`
}

type apiUser struct {
	Username           string   `json:"username"`
	Enabled            bool     `json:"enabled"`
	InRuntime          bool     `json:"inRuntime"`
	CurrentConnections uint64   `json:"currentConnections"`
	ActiveUniqueIPs    uint64   `json:"activeUniqueIps"`
	ActiveIPs          []string `json:"activeIps"`
	TotalOctets        uint64   `json:"totalOctets"`
	MaxUniqueIPs       *uint64  `json:"maxUniqueIps"`
	MaxTCPConns        *uint64  `json:"maxTcpConns"`
	Ips24h             int64    `json:"ips24h"`
	Ips7d              int64    `json:"ips7d"`
	New24h             int64    `json:"new24h"`
	MaxToday           int64    `json:"maxToday"`
	TrafficToday       uint64   `json:"trafficToday"`
	Risk               int      `json:"risk"`
	IPHistory          []apiIP  `json:"ipHistory,omitempty"`
	Link               string   `json:"link,omitempty"`
}

func (s *Server) apiUserView(r *http.Request, u telemt.User) apiUser {
	out := apiUser{
		Username: u.Username, Enabled: u.Enabled, InRuntime: u.InRuntime,
		CurrentConnections: u.CurrentConnections, ActiveUniqueIPs: u.ActiveUniqueIPs,
		ActiveIPs: u.ActiveUniqueIPList, TotalOctets: u.TotalOctets,
		MaxUniqueIPs: u.MaxUniqueIPs, MaxTCPConns: u.MaxTCPConns,
	}
	if s.isAdmin(r) {
		out.Link = u.Link(s.cfg.LinkMode)
	}
	stats, err := s.store.UserIPStats(r.Context(), u.Username, time.Now().UTC())
	if err == nil {
		out.Ips24h = stats.Distinct24h
		out.Ips7d = stats.Distinct7d
		out.New24h = stats.New24h
		out.MaxToday = stats.MaxToday
		out.TrafficToday = stats.TrafficToday
		out.Risk = s.risk(int64(len(u.ActiveUniqueIPList)), stats)
	}
	return out
}

func (s *Server) risk(current int64, stats store.UserIPStats) int {
	switch {
	case current > int64(s.cfg.RiskIPsHigh):
		return 2
	case current > int64(s.cfg.RiskIPsWarn) || stats.Distinct7d > int64(s.cfg.RiskDistinct7d):
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
		if history, err := s.store.IPHistory(r.Context(), name); err == nil {
			for _, item := range history {
				view.IPHistory = append(view.IPHistory, apiIP{
					IP: item.IP, FirstSeen: item.FirstSeen.Format(time.RFC3339),
					LastSeen: item.LastSeen.Format(time.RFC3339), Count: item.Count,
				})
			}
		}
	}
	writeJSON(w, http.StatusOK, view)
}

type createUserRequest struct {
	Name         string  `json:"name"`
	Secret       string  `json:"secret"`
	MaxUniqueIPs *uint64 `json:"maxUniqueIps"`
	MaxTCPConns  *uint64 `json:"maxTcpConns"`
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
	})
	if err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	s.audit(r, "user.create", name, "ok")
	writeJSON(w, http.StatusCreated, s.apiUserView(r, *created))
}

type limitsRequest struct {
	MaxUniqueIPs      *uint64 `json:"maxUniqueIps"`
	MaxTCPConns       *uint64 `json:"maxTcpConns"`
	ClearMaxUniqueIPs bool    `json:"clearMaxUniqueIps"`
	ClearMaxTCPConns  bool    `json:"clearMaxTcpConns"`
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
		if history, err := s.store.IPHistory(r.Context(), name); err == nil {
			for _, item := range history {
				view.IPHistory = append(view.IPHistory, apiIP{
					IP: item.IP, FirstSeen: item.FirstSeen.Format(time.RFC3339),
					LastSeen: item.LastSeen.Format(time.RFC3339), Count: item.Count,
				})
			}
		}
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
		if history, err := s.store.IPHistory(r.Context(), name); err == nil {
			for _, item := range history {
				view.IPHistory = append(view.IPHistory, apiIP{
					IP: item.IP, FirstSeen: item.FirstSeen.Format(time.RFC3339),
					LastSeen: item.LastSeen.Format(time.RFC3339), Count: item.Count,
				})
			}
		}
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
		if history, err := s.store.IPHistory(r.Context(), name); err == nil {
			for _, item := range history {
				view.IPHistory = append(view.IPHistory, apiIP{
					IP: item.IP, FirstSeen: item.FirstSeen.Format(time.RFC3339),
					LastSeen: item.LastSeen.Format(time.RFC3339), Count: item.Count,
				})
			}
		}
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
	link := u.Link(s.cfg.LinkMode)
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
