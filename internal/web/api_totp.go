package web

import (
	"context"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/totp"
	"github.com/go-chi/chi/v5"
	"github.com/skip2/go-qrcode"
)

type totpStatus struct {
	Available    bool   `json:"available"`
	Enabled      bool   `json:"enabled"`
	CreatedAt    string `json:"createdAt,omitempty"`
	RecoveryLeft int    `json:"recoveryLeft"`
}

type totpSetupResponse struct {
	Secret string `json:"secret"`
	URL    string `json:"url"`
	QR     string `json:"qr"`
}

type codeRequest struct {
	Code string `json:"code"`
}

type totpManageRequest struct {
	Password string `json:"password"`
	Code     string `json:"code"`
}

/* ---------- вход ---------- */

type loginTOTPRequest struct {
	Code string `json:"code"`
}

func (s *Server) apiLoginTOTP(w http.ResponseWriter, r *http.Request) {
	var in loginTOTPRequest
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
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
	if err != nil || u.Disabled || !u.TOTPEnabled {
		writeError(w, http.StatusUnauthorized, "unauthorized", "учётная запись недоступна")
		return
	}
	ok, usedCounter, recovery := s.checkTOTPCode(r.Context(), u, in.Code)
	if !ok {
		attempts, _ := s.store.IncrPendingTOTPAttempts(r.Context(), c.Value)
		_ = s.store.RecordLoginFailure(r.Context(), addr.String(), u.Name)
		if attempts >= int64(s.settings.TOTPMaxAttempts(r.Context())) {
			_ = s.store.DeletePendingTOTP(r.Context(), c.Value)
			s.clearCookie(w)
			writeError(w, http.StatusTooManyRequests, "too_many_attempts", "слишком много неверных кодов, начните вход заново")
			return
		}
		writeError(w, http.StatusUnauthorized, "bad_totp", "неверный код")
		return
	}
	if !recovery && usedCounter <= u.TOTPLastCounter {
		writeError(w, http.StatusUnauthorized, "bad_totp", "код уже использован, дождитесь следующего")
		return
	}
	if !recovery {
		_ = s.store.SetTOTPLastCounter(r.Context(), u.Name, usedCounter)
	}
	ttl := s.settings.SessionTTL(r.Context())
	token, err := s.store.CreateSession(r.Context(), u.Name, addr.String(), r.UserAgent(), ttl)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	_ = s.store.DeletePendingTOTP(r.Context(), c.Value)
	_ = s.store.ResetLoginFailures(r.Context(), addr.String(), u.Name)
	result := "totp"
	if recovery {
		result = "recovery"
	}
	_ = s.store.Audit(r.Context(), store.AuditEvent{
		Actor: u.Name, Action: "login." + result, Target: u.Name, Result: "ok", IP: addr.String(),
	})
	s.setCookie(w, token, ttl)
	sess, err := s.store.GetSession(r.Context(), token)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, meResponse{
		User: meUser{Name: u.Name, Role: string(u.Role)}, CSRF: sess.CSRF,
	})
}

// checkTOTPCode проверяет обычный код или резервный.
func (s *Server) checkTOTPCode(ctx context.Context, u *store.User, raw string) (ok bool, counter int64, recovery bool) {
	code := totp.NormalizeCode(raw)
	if code == "" {
		return false, 0, false
	}
	if len(code) == 6 && isDigits(code) {
		secret, err := totp.Decrypt(s.cfg.TOTPKey, u.TOTPSecret)
		if err != nil {
			return false, 0, false
		}
		counter, ok, err := totp.Validate(secret, code, time.Now(), 1)
		if err != nil || !ok {
			return false, 0, false
		}
		return true, counter, false
	}
	used, err := s.store.ConsumeRecoveryCode(ctx, u.Name, totp.HashRecovery(code))
	if err != nil {
		return false, 0, false
	}
	return used, 0, used
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

/* ---------- управление своим вторым фактором ---------- */

func (s *Server) apiTOTPStatus(w http.ResponseWriter, r *http.Request) {
	sess := s.currentSession(r)
	u, err := s.store.GetUser(r.Context(), sess.User)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	out := totpStatus{
		Available: len(s.cfg.TOTPKey) == 32, Enabled: u.TOTPEnabled,
		RecoveryLeft: len(u.RecoveryCodes),
	}
	if u.TOTPEnabled && !u.TOTPCreatedAt.IsZero() {
		out.CreatedAt = u.TOTPCreatedAt.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) apiTOTPSetup(w http.ResponseWriter, r *http.Request) {
	if len(s.cfg.TOTPKey) != 32 {
		writeError(w, http.StatusServiceUnavailable, "totp_unavailable", "2FA недоступна: не задан TGADMIN_TOTP_KEY")
		return
	}
	sess := s.currentSession(r)
	current, err := s.store.GetUser(r.Context(), sess.User)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	if current.TOTPEnabled {
		writeError(w, http.StatusBadRequest, "bad_request", "2FA уже включена: сначала выключите её")
		return
	}
	secret, err := totp.GenerateSecret()
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	encrypted, err := totp.Encrypt(s.cfg.TOTPKey, secret)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	if err := s.store.SetTOTPPending(r.Context(), sess.User, encrypted); err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	url := totp.ProvisioningURL("TeleMT Admin", sess.User, secret)
	png, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, totpSetupResponse{
		Secret: secret, URL: url,
		QR: "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
	})
}

func (s *Server) apiTOTPEnable(w http.ResponseWriter, r *http.Request) {
	sess := s.currentSession(r)
	u, err := s.store.GetUser(r.Context(), sess.User)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	if u.TOTPPending == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "сначала начните настройку")
		return
	}
	var in codeRequest
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	secret, err := totp.Decrypt(s.cfg.TOTPKey, u.TOTPPending)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	counter, ok, err := totp.Validate(secret, in.Code, time.Now(), 1)
	if err != nil || !ok {
		writeError(w, http.StatusBadRequest, "bad_totp", "неверный код, проверьте время на телефоне")
		return
	}
	codes, err := totp.GenerateRecoveryCodes(8)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	hashes := make([]string, 0, len(codes))
	for _, code := range codes {
		hashes = append(hashes, totp.HashRecovery(code))
	}
	encrypted, err := totp.Encrypt(s.cfg.TOTPKey, secret)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	if err := s.store.EnableTOTP(r.Context(), u.Name, encrypted, counter, hashes); err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	if c, err := r.Cookie(cookieName); err == nil {
		_ = s.store.DeleteUserSessions(r.Context(), u.Name, c.Value)
	}
	s.audit(r, "totp.enable", u.Name, "ok")
	writeJSON(w, http.StatusOK, map[string]any{"recoveryCodes": codes})
}

func (s *Server) apiTOTPDisable(w http.ResponseWriter, r *http.Request) {
	sess := s.currentSession(r)
	u, err := s.store.GetUser(r.Context(), sess.User)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	var in totpManageRequest
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	if ok, err := auth.Verify(u.Hash, in.Password); err != nil || !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "неверный пароль")
		return
	}
	if _, ok := s.verifyCurrentTOTP(u, in.Code); !ok {
		writeError(w, http.StatusBadRequest, "bad_totp", "неверный код")
		return
	}
	if err := s.store.DisableTOTP(r.Context(), u.Name); err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	if c, err := r.Cookie(cookieName); err == nil {
		_ = s.store.DeleteUserSessions(r.Context(), u.Name, c.Value)
	}
	s.audit(r, "totp.disable", u.Name, "ok")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiTOTPRecovery(w http.ResponseWriter, r *http.Request) {
	sess := s.currentSession(r)
	u, err := s.store.GetUser(r.Context(), sess.User)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	var in totpManageRequest
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	if ok, err := auth.Verify(u.Hash, in.Password); err != nil || !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "неверный пароль")
		return
	}
	if _, ok := s.verifyCurrentTOTP(u, in.Code); !ok {
		writeError(w, http.StatusBadRequest, "bad_totp", "неверный код")
		return
	}
	codes, err := totp.GenerateRecoveryCodes(8)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	hashes := make([]string, 0, len(codes))
	for _, code := range codes {
		hashes = append(hashes, totp.HashRecovery(code))
	}
	if err := s.store.SetRecoveryCodes(r.Context(), u.Name, hashes); err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	s.audit(r, "totp.recovery", u.Name, "ok")
	writeJSON(w, http.StatusOK, map[string]any{"recoveryCodes": codes})
}

func (s *Server) verifyCurrentTOTP(u *store.User, raw string) (int64, bool) {
	if !u.TOTPEnabled {
		return 0, false
	}
	secret, err := totp.Decrypt(s.cfg.TOTPKey, u.TOTPSecret)
	if err != nil {
		return 0, false
	}
	counter, ok, err := totp.Validate(secret, raw, time.Now(), 1)
	if err != nil || !ok || counter <= u.TOTPLastCounter {
		return 0, false
	}
	return counter, true
}

/* ---------- сброс администратором ---------- */

func (s *Server) apiResetAdminTOTP(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if !nameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "недопустимое имя")
		return
	}
	sess := s.currentSession(r)
	if sess.User == name {
		writeError(w, http.StatusBadRequest, "bad_request", "свой второй фактор отключается в аккаунте с паролем и кодом")
		return
	}
	u, err := s.store.GetUser(r.Context(), name)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	if !u.TOTPEnabled {
		writeError(w, http.StatusBadRequest, "bad_request", "у пользователя не включена 2FA")
		return
	}
	if err := s.store.DisableTOTP(r.Context(), name); err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	_ = s.store.DeleteUserSessions(r.Context(), name, "")
	s.audit(r, "admin.totp.reset", name, "ok")
	w.WriteHeader(http.StatusNoContent)
}
