package web

import (
	"net/http"
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
	addr, _ := clientAddr(r)
	allowed, err := s.store.LoginAllowed(r.Context(), addr.String(), username)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	if !allowed {
		writeError(w, http.StatusTooManyRequests, "too_many_attempts", "слишком много попыток, попробуйте через 15 минут")
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
	token, err := s.store.CreateSession(r.Context(), username, addr.String(), r.UserAgent(), s.cfg.SessionTTL)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	_ = s.store.ResetLoginFailures(r.Context(), addr.String(), username)
	_ = s.store.Audit(r.Context(), store.AuditEvent{
		Actor: username, Action: "login", Target: username, Result: "ok", IP: addr.String(),
	})
	s.setCookie(w, token)
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
