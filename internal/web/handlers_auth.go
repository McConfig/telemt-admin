package web

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/store"
)

type loginData struct {
	Title   string
	Error   string
	NoUsers bool
}

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	n, err := s.store.CountUsers(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, "login.html", loginData{Title: "Вход", NoUsers: n == 0})
}

func (s *Server) loginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "неверная форма", http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(r.PostFormValue("username"))
	password := r.PostFormValue("password")
	addr, _ := clientAddr(r)

	allowed, err := s.store.LoginAllowed(r.Context(), addr.String(), username)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if !allowed {
		http.Error(w, "слишком много попыток, попробуйте через 15 минут", http.StatusTooManyRequests)
		return
	}

	u, err := s.store.GetUser(r.Context(), username)
	if err != nil {
		_ = s.store.RecordLoginFailure(r.Context(), addr.String(), username)
		http.Error(w, "неверное имя или пароль", http.StatusUnauthorized)
		return
	}
	if u.Disabled {
		http.Error(w, "учётная запись отключена", http.StatusUnauthorized)
		return
	}
	ok, err := auth.Verify(u.Hash, password)
	if err != nil || !ok {
		_ = s.store.RecordLoginFailure(r.Context(), addr.String(), username)
		http.Error(w, "неверное имя или пароль", http.StatusUnauthorized)
		return
	}
	token, err := s.store.CreateSession(r.Context(), username, addr.String(), r.UserAgent(), s.cfg.SessionTTL)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	_ = s.store.ResetLoginFailures(r.Context(), addr.String(), username)
	_ = s.store.Audit(r.Context(), store.AuditEvent{
		Actor: username, Action: "login", Target: username, Result: "ok", IP: addr.String(),
	})
	s.setCookie(w, token)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if !s.checkCSRF(w, r) {
		return
	}
	if sess := s.currentSession(r); sess != nil {
		_ = s.store.DeleteSession(r.Context(), cookieValue(r))
		_ = s.store.Audit(r.Context(), store.AuditEvent{
			Actor: sess.User, Action: "logout", Target: sess.User, Result: "ok",
		})
	}
	s.clearCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) setCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: token, Path: "/",
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
		MaxAge: int(s.cfg.SessionTTL.Seconds()),
	})
}

func (s *Server) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) redirectLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func cookieValue(r *http.Request) string {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

func (s *Server) checkCSRF(w http.ResponseWriter, r *http.Request) bool {
	sess := s.currentSession(r)
	if sess == nil {
		s.redirectLogin(w, r)
		return false
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "неверная форма", http.StatusBadRequest)
		return false
	}
	if r.PostFormValue("csrf") != sess.CSRF {
		http.Error(w, "устаревшая форма, обновите страницу", http.StatusForbidden)
		return false
	}
	origin := r.Header.Get("Origin")
	if origin != "" && origin != s.cfg.BaseURL.Scheme+"://"+s.cfg.BaseURL.Host {
		http.Error(w, "недопустимый источник", http.StatusForbidden)
		return false
	}
	return true
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		s.serverError(w, r, err)
	}
}

func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("внутренняя ошибка", "path", r.URL.Path, "err", err)
	http.Error(w, "внутренняя ошибка", http.StatusInternalServerError)
}
