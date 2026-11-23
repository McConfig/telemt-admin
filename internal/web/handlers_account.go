package web

import (
	"net/http"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/go-chi/chi/v5"
)

type auditData struct {
	Title  string
	Events []store.AuditEvent
	CSRF   string
}

func (s *Server) auditPage(w http.ResponseWriter, r *http.Request) {
	events, err := s.store.RecentAudit(r.Context(), 200)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	sess := s.currentSession(r)
	csrf := ""
	if sess != nil {
		csrf = sess.CSRF
	}
	s.render(w, r, "audit.html", auditData{Title: "Аудит", Events: events, CSRF: csrf})
}

type accountData struct {
	Title     string
	User      *store.User
	Sessions  []store.Session
	Error     string
	CSRF      string
	CurrentID string
}

func (s *Server) accountPage(w http.ResponseWriter, r *http.Request) {
	sess := s.currentSession(r)
	u, err := s.store.GetUser(r.Context(), sess.User)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	sessions, err := s.store.ListSessions(r.Context(), sess.User)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, "account.html", accountData{
		Title: "Аккаунт", User: u, Sessions: sessions,
		CSRF: sess.CSRF, CurrentID: sess.ID,
	})
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	if !s.checkCSRF(w, r) {
		return
	}
	sess := s.currentSession(r)
	current := r.PostFormValue("current")
	newPassword := r.PostFormValue("new")
	u, err := s.store.GetUser(r.Context(), sess.User)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	ok, err := auth.Verify(u.Hash, current)
	if err != nil || !ok {
		http.Error(w, "текущий пароль неверен", http.StatusForbidden)
		return
	}
	if err := auth.CheckPolicy(newPassword); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	hash, err := auth.Hash(newPassword)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if err := s.store.SetPassword(r.Context(), sess.User, hash); err != nil {
		s.serverError(w, r, err)
		return
	}
	if err := s.store.DeleteUserSessions(r.Context(), sess.User, cookieValue(r)); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.audit(r, "account.password", sess.User, "ok")
	http.Redirect(w, r, "/account", http.StatusSeeOther)
}

func (s *Server) revokeSession(w http.ResponseWriter, r *http.Request) {
	if !s.checkCSRF(w, r) {
		return
	}
	sess := s.currentSession(r)
	id := chi.URLParam(r, "id")
	if id == sess.ID {
		http.Error(w, "нельзя завершить текущую сессию", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteSessionByID(r.Context(), sess.User, id); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.audit(r, "account.revoke", sess.User, "ok")
	http.Redirect(w, r, "/account", http.StatusSeeOther)
}
