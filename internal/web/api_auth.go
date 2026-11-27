package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/version"
)

type require2FAInfo struct {
	Enabled  bool   `json:"enabled"`
	Enforced bool   `json:"enforced"`
	Deadline string `json:"deadline,omitempty"`
	DaysLeft int    `json:"daysLeft"`
}

type meResponse struct {
	User       meUser          `json:"user"`
	CSRF       string          `json:"csrf"`
	Require2FA *require2FAInfo `json:"require2fa,omitempty"`
	Service    bool            `json:"service,omitempty"`
	Scope      string          `json:"scope,omitempty"`
}

type meUser struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

func (s *Server) me(r *http.Request) (meResponse, error) {
	p := s.currentPrincipal(r)
	if p == nil {
		return meResponse{}, store.ErrNotFound
	}
	if p.Kind == "service" {
		return meResponse{
			User:    meUser{Name: p.Name, Role: string(p.Role)},
			Service: true, Scope: p.Scope,
		}, nil
	}
	u, err := s.store.GetUser(r.Context(), p.Name)
	if err != nil {
		return meResponse{}, err
	}
	return s.meForUser(r.Context(), *u, p.Session.CSRF), nil
}

func (s *Server) meForUser(ctx context.Context, u store.User, csrf string) meResponse {
	out := meResponse{User: meUser{Name: u.Name, Role: string(u.Role)}, CSRF: csrf}
	if u.Role != store.RoleAdmin || s.hasSecondFactor(ctx, u) {
		return out
	}
	enabled, deadline, daysLeft := s.settings.Require2FA(ctx)
	if !enabled {
		return out
	}
	out.Require2FA = &require2FAInfo{
		Enabled: true, Enforced: time.Now().UTC().After(deadline),
		Deadline: deadline.UTC().Format(time.RFC3339), DaysLeft: daysLeft,
	}
	return out
}

func (s *Server) apiMe(w http.ResponseWriter, r *http.Request) {
	m, err := s.me(r)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) apiMeta(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version":          version.Version,
		"repo":             "https://github.com/McConfig/telemt-admin",
		"defaultChartDays": s.settings.DefaultChartDays(r.Context()),
	})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) apiLogin(w http.ResponseWriter, r *http.Request) {
	var in loginRequest
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	username := strings.TrimSpace(in.Username)
	addr, _ := s.clientAddr(r)
	allowed, err := s.store.LoginAllowed(r.Context(), addr.String(), username)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	if !allowed {
		writeError(w, http.StatusTooManyRequests, "too_many_attempts", "слишком много попыток, попробуйте позже")
		return
	}
	u, err := s.store.GetUser(r.Context(), username)
	if err != nil || u.Disabled {
		_ = s.store.RecordLoginFailure(r.Context(), addr.String(), username)
		writeError(w, http.StatusUnauthorized, "unauthorized", "неверное имя или пароль")
		return
	}
	ok, err := auth.Verify(u.Hash, in.Password)
	if err != nil || !ok {
		_ = s.store.RecordLoginFailure(r.Context(), addr.String(), username)
		writeError(w, http.StatusUnauthorized, "unauthorized", "неверное имя или пароль")
		return
	}
	passkeyCount, _ := s.store.CountPasskeys(r.Context(), username)
	if u.TOTPEnabled || passkeyCount > 0 {
		if u.TOTPEnabled && passkeyCount == 0 && len(s.cfg.TOTPKey) != 32 {
			writeError(w, http.StatusServiceUnavailable, "totp_unavailable", "2FA включена, но на сервере не задан TGADMIN_TOTP_KEY")
			return
		}
		pendingTTL := s.settings.TOTPPendingTTL(r.Context())
		pending, err := s.store.CreatePendingTOTP(r.Context(), username, addr.String(), r.UserAgent(), pendingTTL)
		if err != nil {
			s.serverErrorJSON(w, r, err)
			return
		}
		_ = s.store.ResetLoginFailures(r.Context(), addr.String(), username)
		_ = s.store.Audit(r.Context(), store.AuditEvent{
			Actor: username, Action: "login.mfa.required", Target: username, Result: "ok", IP: addr.String(),
		})
		s.setCookie(w, pending, pendingTTL)
		resp := map[string]any{
			"mfaRequired": true,
			"passkey":     passkeyCount > 0,
			"totp":        u.TOTPEnabled,
		}
		if u.TOTPEnabled {
			resp["totpRequired"] = true
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	ttl := s.settings.SessionTTL(r.Context())
	token, err := s.store.CreateSession(r.Context(), username, addr.String(), r.UserAgent(), ttl)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	_ = s.store.ResetLoginFailures(r.Context(), addr.String(), username)
	_ = s.store.Audit(r.Context(), store.AuditEvent{
		Actor: username, Action: "login", Target: username, Result: "ok", IP: addr.String(),
	})
	s.setCookie(w, token, ttl)
	sess, err := s.store.GetSession(r.Context(), token)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, s.meForUser(r.Context(), *u, sess.CSRF))
}

func (s *Server) apiLogout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(cookieName)
	if err == nil {
		_ = s.store.DeleteSession(r.Context(), c.Value)
	}
	if sess := s.currentSession(r); sess != nil {
		s.audit(r, "logout", sess.User, "ok")
	}
	s.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}
