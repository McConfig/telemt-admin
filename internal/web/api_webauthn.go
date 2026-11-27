package web

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/go-chi/chi/v5"
)

type passkeyView struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CreatedAt  string `json:"createdAt"`
	LastUsedAt string `json:"lastUsedAt,omitempty"`
}

func (s *Server) apiPasskeys(w http.ResponseWriter, r *http.Request) {
	sess := s.currentSession(r)
	if sess == nil {
		writeError(w, http.StatusForbidden, "session_required", "доступно только из интерфейса")
		return
	}
	list, err := s.store.ListPasskeys(r.Context(), sess.User)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	out := make([]passkeyView, 0, len(list))
	for _, p := range list {
		view := passkeyView{
			ID: p.ID, Name: p.Name,
			CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339),
		}
		if !p.LastUsedAt.IsZero() {
			view.LastUsedAt = p.LastUsedAt.UTC().Format(time.RFC3339)
		}
		out = append(out, view)
	}
	writeJSON(w, http.StatusOK, out)
}

type passkeyRegisterBeginRequest struct {
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (s *Server) apiPasskeyRegisterBegin(w http.ResponseWriter, r *http.Request) {
	sess := s.currentSession(r)
	if sess == nil {
		writeError(w, http.StatusForbidden, "session_required", "доступно только из интерфейса")
		return
	}
	if s.passkeys == nil {
		writeErrorHint(w, http.StatusServiceUnavailable, "webauthn_unavailable",
			"WebAuthn недоступен", "проверьте HTTPS и TGADMIN_BASE_URL")
		return
	}
	var in passkeyRegisterBeginRequest
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	u, err := s.store.GetUser(r.Context(), sess.User)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	ok, err := auth.Verify(u.Hash, in.Password)
	if err != nil || !ok {
		writeError(w, http.StatusForbidden, "forbidden", "текущий пароль неверен")
		return
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = "Passkey"
	}
	if utf8.RuneCountInString(name) > 64 {
		writeError(w, http.StatusBadRequest, "bad_request", "имя: не больше 64 символов")
		return
	}
	options, err := s.passkeys.BeginRegistration(r.Context(), sess.User, sess.ID, name)
	if err != nil {
		if errors.Is(err, store.ErrLimit) {
			writeError(w, http.StatusTooManyRequests, "too_many", "не больше 10 passkeys")
			return
		}
		s.serverErrorJSON(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, options)
}

func (s *Server) apiPasskeyRegisterFinish(w http.ResponseWriter, r *http.Request) {
	sess := s.currentSession(r)
	if sess == nil {
		writeError(w, http.StatusForbidden, "session_required", "доступно только из интерфейса")
		return
	}
	if s.passkeys == nil {
		writeErrorHint(w, http.StatusServiceUnavailable, "webauthn_unavailable",
			"WebAuthn недоступен", "проверьте HTTPS и TGADMIN_BASE_URL")
		return
	}
	p, err := s.passkeys.FinishRegistration(r.Context(), sess.User, sess.ID, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_passkey", "не удалось проверить passkey")
		return
	}
	s.audit(r, "passkey.create", p.Name, p.ID)
	view := passkeyView{ID: p.ID, Name: p.Name, CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339)}
	writeJSON(w, http.StatusCreated, view)
}

func (s *Server) apiPasskeyDelete(w http.ResponseWriter, r *http.Request) {
	sess := s.currentSession(r)
	if sess == nil {
		writeError(w, http.StatusForbidden, "session_required", "доступно только из интерфейса")
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	u, err := s.store.GetUser(r.Context(), sess.User)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	ok, err := auth.Verify(u.Hash, in.Password)
	if err != nil || !ok {
		writeError(w, http.StatusForbidden, "forbidden", "текущий пароль неверен")
		return
	}
	id := chi.URLParam(r, "id")
	count, err := s.store.CountPasskeys(r.Context(), sess.User)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	if count <= 1 && !u.TOTPEnabled {
		if enabled, _, _ := s.settings.Require2FA(r.Context()); enabled {
			writeErrorHint(w, http.StatusConflict, "last_factor",
				"нельзя удалить последний второй фактор", "сначала включите TOTP")
			return
		}
	}
	if err := s.store.DeletePasskey(r.Context(), sess.User, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "passkey не найден")
			return
		}
		s.serverErrorJSON(w, r, err)
		return
	}
	s.audit(r, "passkey.delete", id, "ok")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiPasskeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	if s.passkeys == nil {
		writeErrorHint(w, http.StatusServiceUnavailable, "webauthn_unavailable",
			"WebAuthn недоступен", "проверьте HTTPS и TGADMIN_BASE_URL")
		return
	}
	c, err := r.Cookie(cookieName)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "сессия входа истекла")
		return
	}
	pending, err := s.store.GetPendingTOTP(r.Context(), c.Value)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "сессия входа истекла, начните заново")
		return
	}
	options, err := s.passkeys.BeginLogin(r.Context(), pending.User, c.Value)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "bad_passkey", "у пользователя нет passkeys")
			return
		}
		s.serverErrorJSON(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, options)
}

func (s *Server) apiPasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	if s.passkeys == nil {
		writeErrorHint(w, http.StatusServiceUnavailable, "webauthn_unavailable",
			"WebAuthn недоступен", "проверьте HTTPS и TGADMIN_BASE_URL")
		return
	}
	c, err := r.Cookie(cookieName)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "сессия входа истекла")
		return
	}
	pending, err := s.store.GetPendingTOTP(r.Context(), c.Value)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "сессия входа истекла, начните заново")
		return
	}
	addr, _ := s.clientAddr(r)
	allowed, err := s.store.LoginAllowed(r.Context(), addr.String(), pending.User)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	if !allowed {
		writeError(w, http.StatusTooManyRequests, "too_many_attempts", "слишком много попыток, попробуйте позже")
		return
	}
	u, err := s.store.GetUser(r.Context(), pending.User)
	if err != nil || u.Disabled {
		writeError(w, http.StatusUnauthorized, "unauthorized", "учётная запись недоступна")
		return
	}
	if _, err := s.passkeys.FinishLogin(r.Context(), pending.User, c.Value, r); err != nil {
		_, _ = s.store.IncrPendingTOTPAttempts(r.Context(), c.Value)
		writeError(w, http.StatusUnauthorized, "bad_passkey", "не удалось проверить passkey")
		return
	}
	_ = s.store.DeletePendingTOTP(r.Context(), c.Value)
	ttl := s.settings.SessionTTL(r.Context())
	token, err := s.store.CreateSession(r.Context(), u.Name, addr.String(), r.UserAgent(), ttl)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	_ = s.store.ResetLoginFailures(r.Context(), addr.String(), u.Name)
	_ = s.store.Audit(r.Context(), store.AuditEvent{
		Actor: u.Name, Action: "login.webauthn", Target: u.Name, Result: "ok", IP: addr.String(),
	})
	s.setCookie(w, token, ttl)
	sess, err := s.store.GetSession(r.Context(), token)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, s.meForUser(r.Context(), *u, sess.CSRF))
}
