package web

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
)

type ctxKey int

const sessionCtxKey ctxKey = 1

const cookieName = "tgadmin_session"

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
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

func writeErrorHint(w http.ResponseWriter, code int, kind, msg, hint string) {
	writeJSON(w, code, errorEnvelope{Error: errorBody{
		Code: kind, Message: msg, Hint: hint,
	}})
}

func (s *Server) withAPISession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if raw, present := bearerToken(r); present {
			auth, err := s.store.AuthenticateServiceToken(r.Context(), raw)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid_token", "неверный или истёкший токен")
				return
			}
			if !servicePathAllowed(string(auth.Service.Scope), r.URL.Path) {
				writeError(w, http.StatusForbidden, "session_required",
					"этот раздел доступен только из интерфейса")
				return
			}
			p := &principal{
				Name: "service:" + auth.Service.Name, Kind: "service",
				Scope: string(auth.Service.Scope), TokenID: auth.TokenID,
				Role: roleForScope(auth.Service.Scope),
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalCtxKey, p)))
			return
		}
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
		if s.totpRestricted(r.Context(), *u) && !totpAllowedPath(r.URL.Path) {
			writeError(w, http.StatusForbidden, "totp_required", "включите двухфакторную аутентификацию")
			return
		}
		p := &principal{Name: u.Name, Kind: "session", Role: u.Role, Session: sess}
		ctx := context.WithValue(r.Context(), sessionCtxKey, sess)
		ctx = context.WithValue(ctx, principalCtxKey, p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// bearerToken возвращает токен только для схемы Bearer. Другие схемы
// игнорируются, чтобы Basic-auth от прокси не ломал cookie-сессию.
func bearerToken(r *http.Request) (string, bool) {
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "bearer "
	if len(raw) <= len(prefix) || !strings.EqualFold(raw[:len(prefix)], prefix) {
		return "", false
	}
	return strings.TrimSpace(raw[len(prefix):]), true
}

func (s *Server) apiCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := s.currentPrincipal(r); p != nil && p.Kind == "service" {
			next.ServeHTTP(w, r)
			return
		}
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
	p := s.currentPrincipal(r)
	if p == nil {
		return false
	}
	if p.Kind == "service" {
		if p.Scope == string(store.ScopeAdmin) {
			return true
		}
		if p.Scope == string(store.ScopeWrite) {
			return r.URL.Path == "/api/users" || strings.HasPrefix(r.URL.Path, "/api/users/")
		}
		return false
	}
	return p.Role == store.RoleAdmin
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
		code := apiErr.Code
		if code == "" {
			code = telemt.CodeAPIError
		}
		status := http.StatusBadGateway
		switch apiErr.Status {
		case http.StatusBadRequest, http.StatusUnprocessableEntity:
			status = http.StatusBadRequest
		case http.StatusNotFound:
			status = http.StatusNotFound
		case http.StatusConflict:
			status = http.StatusConflict
		}
		slog.Warn("telemt API", "method", r.Method, "path", r.URL.Path,
			"status", apiErr.Status, "code", code)
		writeErrorHint(w, status, code, apiErr.Message, telemt.Hint(code))
		return
	}
	slog.Warn("telemt недоступен", "method", r.Method, "path", r.URL.Path, "err", err)
	writeErrorHint(w, http.StatusBadGateway, telemt.CodeUnreachable,
		"нет соединения с Control API", telemt.Hint(telemt.CodeUnreachable))
}

func (s *Server) setCookie(w http.ResponseWriter, token string, ttl time.Duration) {
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
	actor := s.actor(r)
	addr, _ := s.clientAddr(r)
	_ = s.store.Audit(r.Context(), store.AuditEvent{
		Actor: actor, Action: action, Target: target, Result: result, IP: addr.String(),
	})
	if s.notifier != nil {
		s.notifier.Emit(r.Context(), action, map[string]any{
			"actor": actor, "target": target, "result": result,
			"ip": addr.String(), "at": time.Now().UTC().Format(time.RFC3339),
		})
	}
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

func (s *Server) totpRestricted(ctx context.Context, u store.User) bool {
	if u.Role != store.RoleAdmin || u.TOTPEnabled {
		return false
	}
	enabled, deadline, _ := s.settings.Require2FA(ctx)
	return enabled && time.Now().UTC().After(deadline)
}

func totpAllowedPath(path string) bool {
	switch path {
	case "/api/me", "/api/logout", "/api/account/password",
		"/api/account/totp", "/api/account/totp/setup", "/api/account/totp/enable":
		return true
	}
	return strings.HasPrefix(path, "/api/account/sessions")
}
