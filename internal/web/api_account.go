package web

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/store"
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
	events, err := s.store.RecentAudit(r.Context(), 10000)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	q := r.URL.Query()
	actor := strings.ToLower(strings.TrimSpace(q.Get("actor")))
	action := strings.ToLower(strings.TrimSpace(q.Get("action")))
	target := strings.ToLower(strings.TrimSpace(q.Get("target")))
	search := strings.ToLower(strings.TrimSpace(q.Get("q")))
	from, to := s.parseAuditTime(q.Get("from")), s.parseAuditTime(q.Get("to"))
	limit := 200
	if raw := q.Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	offset := 0
	if raw := q.Get("offset"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
			offset = n
		}
	}
	filtered := make([]store.AuditEvent, 0, len(events))
	for _, e := range events {
		if actor != "" && !strings.Contains(strings.ToLower(e.Actor), actor) {
			continue
		}
		if action != "" && e.Action != action {
			continue
		}
		if target != "" && !strings.Contains(strings.ToLower(e.Target), target) {
			continue
		}
		if search != "" {
			line := strings.ToLower(e.Actor + " " + e.Action + " " + e.Target + " " + e.IP)
			if !strings.Contains(line, search) {
				continue
			}
		}
		if !from.IsZero() && e.At.Before(from) {
			continue
		}
		if !to.IsZero() && e.At.After(to.Add(24*time.Hour)) {
			continue
		}
		filtered = append(filtered, e)
	}
	if offset >= len(filtered) {
		filtered = nil
	} else {
		filtered = filtered[offset:]
	}
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	out := make([]auditItem, 0, len(filtered))
	for _, e := range filtered {
		out = append(out, auditItem{
			At: e.At.UTC().Format("2006-01-02T15:04:05Z"), Actor: e.Actor,
			Action: e.Action, Target: e.Target, Result: e.Result, IP: e.IP,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) parseAuditTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t
	}
	return time.Time{}
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
