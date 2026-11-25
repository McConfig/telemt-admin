package web

import (
	"net/http"

	"github.com/McConfig/telemt-admin/internal/version"
	"strings"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/store"
)

type meResponse struct {
	User meUser `json:"user"`
	CSRF string `json:"csrf"`
}

type meUser struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

func (s *Server) me(r *http.Request) (meResponse, error) {
	sess := s.currentSession(r)
	u, err := s.store.GetUser(r.Context(), sess.User)
	if err != nil {
		return meResponse{}, err
	}
	return meResponse{User: meUser{Name: u.Name, Role: string(u.Role)}, CSRF: sess.CSRF}, nil
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
	if u.TOTPEnabled {
		if len(s.cfg.TOTPKey) != 32 {
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
			Actor: username, Action: "login.totp.required", Target: username, Result: "ok", IP: addr.String(),
		})
		s.setCookie(w, pending, pendingTTL)
		writeJSON(w, http.StatusOK, map[string]any{"totpRequired": true})
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
	writeJSON(w, http.StatusOK, meResponse{
		User: meUser{Name: username, Role: string(u.Role)}, CSRF: sess.CSRF,
	})
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
