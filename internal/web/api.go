package web

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
)

type ctxKey int

const sessionCtxKey ctxKey = 1

const cookieName = "tgadmin_session"

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, kind, msg string) {
	writeJSON(w, code, errorEnvelope{Error: errorBody{Code: kind, Message: msg}})
}

func (s *Server) withAPISession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "требуется вход")
			return
		}
		sess, err := s.store.GetSession(r.Context(), c.Value)
		if err != nil {
			s.clearCookie(w)
			writeError(w, http.StatusUnauthorized, "unauthorized", "сессия истекла")
			return
		}
		u, err := s.store.GetUser(r.Context(), sess.User)
		if err != nil || u.Disabled {
			s.clearCookie(w)
			writeError(w, http.StatusUnauthorized, "unauthorized", "учётная запись недоступна")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionCtxKey, sess)))
	})
}

func (s *Server) apiCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		sess := s.currentSession(r)
		if sess == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "требуется вход")
			return
		}
		token := r.Header.Get("X-CSRF-Token")
		if subtle.ConstantTimeCompare([]byte(token), []byte(sess.CSRF)) != 1 {
			writeError(w, http.StatusForbidden, "csrf", "устаревший CSRF-токен, обновите страницу")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != s.cfg.BaseURL.Scheme+"://"+s.cfg.BaseURL.Host {
			writeError(w, http.StatusForbidden, "origin", "недопустимый источник")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) currentSession(r *http.Request) *store.Session {
	sess, _ := r.Context().Value(sessionCtxKey).(*store.Session)
	return sess
}

func (s *Server) isAdmin(r *http.Request) bool {
	sess := s.currentSession(r)
	if sess == nil {
		return false
	}
	u, err := s.store.GetUser(r.Context(), sess.User)
	return err == nil && u.Role == store.RoleAdmin
}

func (s *Server) requireAdminAPI(w http.ResponseWriter, r *http.Request) bool {
	if !s.isAdmin(r) {
		writeError(w, http.StatusForbidden, "forbidden", "нет прав")
		return false
	}
	return true
}

func (s *Server) telemtAPIError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *telemt.APIError
	if errors.As(err, &apiErr) {
		slog.Warn("telemt API", "method", r.Method, "path", r.URL.Path, "status", apiErr.Status)
		if apiErr.Status >= http.StatusInternalServerError {
			writeError(w, http.StatusBadGateway, "telemt_unavailable", "telemt не отвечает")
			return
		}
		code := http.StatusBadGateway
		switch apiErr.Status {
		case http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound, http.StatusConflict:
			code = apiErr.Status
		}
		writeError(w, code, "telemt", apiErr.Message)
		return
	}
	slog.Warn("telemt недоступен", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusBadGateway, "telemt_unavailable", "telemt не отвечает")
}

func (s *Server) setCookie(w http.ResponseWriter, token string) {
	// #nosec G124 -- Secure управляется TGADMIN_COOKIE_SECURE, по умолчанию true.
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: token, Path: "/",
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
		MaxAge: int(s.cfg.SessionTTL.Seconds()),
	})
}

func (s *Server) clearCookie(w http.ResponseWriter) {
	// #nosec G124 -- Secure управляется TGADMIN_COOKIE_SECURE, по умолчанию true.
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) audit(r *http.Request, action, target, result string) {
	sess := s.currentSession(r)
	actor := ""
	if sess != nil {
		actor = sess.User
	}
	addr, _ := s.clientAddr(r)
	_ = s.store.Audit(r.Context(), store.AuditEvent{
		Actor: actor, Action: action, Target: target, Result: result, IP: addr.String(),
	})
}

func decodeJSON(r *http.Request, v any) error {
	defer func() { _ = r.Body.Close() }()
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	return dec.Decode(v)
}

func (s *Server) serverErrorJSON(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("внутренняя ошибка", "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal", "внутренняя ошибка")
}
