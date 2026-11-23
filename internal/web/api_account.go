package web

import (
	"encoding/csv"
	"net/http"
	"time"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/go-chi/chi/v5"
)

type auditItem struct {
	At     string `json:"at"`
	Actor  string `json:"actor"`
	Action string `json:"action"`
	Target string `json:"target"`
	Result string `json:"result"`
	IP     string `json:"ip"`
}

func (s *Server) apiAudit(w http.ResponseWriter, r *http.Request) {
	events, err := s.store.RecentAudit(r.Context(), 200)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	out := make([]auditItem, 0, len(events))
	for _, e := range events {
		out = append(out, auditItem{
			At: e.At.UTC().Format("2006-01-02T15:04:05Z"), Actor: e.Actor,
			Action: e.Action, Target: e.Target, Result: e.Result, IP: e.IP,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type sessionItem struct {
	ID        string `json:"id"`
	UserAgent string `json:"userAgent"`
	IP        string `json:"ip"`
	CreatedAt string `json:"createdAt"`
	Current   bool   `json:"current"`
}

func (s *Server) apiSessions(w http.ResponseWriter, r *http.Request) {
	sess := s.currentSession(r)
	list, err := s.store.ListSessions(r.Context(), sess.User)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	out := make([]sessionItem, 0, len(list))
	for _, item := range list {
		out = append(out, sessionItem{
			ID: item.ID, UserAgent: item.UserAgent, IP: item.IP,
			CreatedAt: item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			Current:   item.ID == sess.ID,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type passwordRequest struct {
	Current string `json:"current"`
	New     string `json:"new"`
}

func (s *Server) apiChangePassword(w http.ResponseWriter, r *http.Request) {
	sess := s.currentSession(r)
	var in passwordRequest
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	u, err := s.store.GetUser(r.Context(), sess.User)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	ok, err := auth.Verify(u.Hash, in.Current)
	if err != nil || !ok {
		writeError(w, http.StatusForbidden, "forbidden", "текущий пароль неверен")
		return
	}
	if err := auth.CheckPolicy(in.New); err != nil {
		writeError(w, http.StatusBadRequest, "policy", err.Error())
		return
	}
	hash, err := auth.Hash(in.New)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	if err := s.store.SetPassword(r.Context(), sess.User, hash); err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	if err := s.store.DeleteUserSessions(r.Context(), sess.User, cookieValue(r)); err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	s.audit(r, "account.password", sess.User, "ok")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiRevokeSession(w http.ResponseWriter, r *http.Request) {
	sess := s.currentSession(r)
	id := chi.URLParam(r, "id")
	if id == sess.ID {
		writeError(w, http.StatusBadRequest, "bad_request", "нельзя завершить текущую сессию")
		return
	}
	if err := s.store.DeleteSessionByID(r.Context(), sess.User, id); err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	s.audit(r, "account.revoke", sess.User, "ok")
	w.WriteHeader(http.StatusNoContent)
}

func cookieValue(r *http.Request) string {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

func (s *Server) apiAuditCSV(w http.ResponseWriter, r *http.Request) {
	events, err := s.store.RecentAudit(r.Context(), 10000)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"audit.csv\"")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"at", "actor", "action", "target", "result", "ip"})
	for _, e := range events {
		_ = cw.Write([]string{e.At.Format(time.RFC3339), e.Actor, e.Action, e.Target, e.Result, e.IP})
	}
	cw.Flush()
}
