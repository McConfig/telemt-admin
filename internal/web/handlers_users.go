package web

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
	"github.com/go-chi/chi/v5"
	"github.com/skip2/go-qrcode"
)

type userRow struct {
	telemt.User
	Link string
}

type usersData struct {
	Title string
	Users []userRow
	Query string
}

func (s *Server) usersPage(w http.ResponseWriter, r *http.Request) {
	users, err := s.telemt.ListUsers(r.Context())
	if err != nil {
		s.telemtError(w, r, err)
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	rows := make([]userRow, 0, len(users))
	for _, u := range users {
		if q != "" && !strings.Contains(strings.ToLower(u.Username), q) {
			continue
		}
		rows = append(rows, userRow{User: u, Link: u.Link(s.cfg.LinkMode)})
	}
	s.render(w, r, "users.html", usersData{Title: "Пользователи", Users: rows, Query: q})
}

func (s *Server) telemtError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *telemt.APIError
	if errors.As(err, &apiErr) {
		slog.Warn("telemt API", "method", r.Method, "path", r.URL.Path, "status", apiErr.Status)
		if apiErr.Status >= http.StatusInternalServerError {
			http.Error(w, "telemt не отвечает", http.StatusBadGateway)
			return
		}
		code := http.StatusBadGateway
		switch apiErr.Status {
		case http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound, http.StatusConflict:
			code = apiErr.Status
		}
		http.Error(w, "telemt API: "+apiErr.Message, code)
		return
	}
	slog.Warn("telemt недоступен", "method", r.Method, "path", r.URL.Path, "err", err)
	http.Error(w, "telemt не отвечает", http.StatusBadGateway)
}

type userData struct {
	Title     string
	User      *telemt.User
	Link      string
	CanManage bool
	CSRF      string
}

func (s *Server) userPage(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	u, err := s.telemt.GetUser(r.Context(), name)
	if err != nil {
		s.telemtError(w, r, err)
		return
	}
	sess := s.currentSession(r)
	data := userData{
		Title: name, User: u, Link: u.Link(s.cfg.LinkMode),
		CanManage: s.isAdmin(r),
	}
	if sess != nil {
		data.CSRF = sess.CSRF
	}
	s.render(w, r, "user.html", data)
}

func (s *Server) userQR(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		http.Error(w, "нет прав", http.StatusForbidden)
		return
	}
	name := chi.URLParam(r, "name")
	u, err := s.telemt.GetUser(r.Context(), name)
	if err != nil {
		s.telemtError(w, r, err)
		return
	}
	link := u.Link(s.cfg.LinkMode)
	if link == "" {
		http.Error(w, "у пользователя нет ссылки", http.StatusNotFound)
		return
	}
	img, err := qrcode.Encode(link, qrcode.Medium, 256)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(img)
}

func (s *Server) isAdmin(r *http.Request) bool {
	sess := s.currentSession(r)
	if sess == nil {
		return false
	}
	u, err := s.store.GetUser(r.Context(), sess.User)
	return err == nil && u.Role == store.RoleAdmin
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
var secretRe = regexp.MustCompile(`^[0-9a-f]{32}$`)

type newUserData struct {
	Title string
	Error string
	CSRF  string
}

func (s *Server) newUserPage(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		http.Error(w, "нет прав", http.StatusForbidden)
		return
	}
	sess := s.currentSession(r)
	csrf := ""
	if sess != nil {
		csrf = sess.CSRF
	}
	s.render(w, r, "new.html", newUserData{Title: "Новый пользователь", CSRF: csrf})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) || !s.checkCSRF(w, r) {
		return
	}
	name := strings.TrimSpace(r.PostFormValue("name"))
	if !nameRe.MatchString(name) {
		http.Error(w, "имя: латиница, цифры, точка, дефис, подчёркивание, до 64 символов", http.StatusBadRequest)
		return
	}
	in := telemt.CreateInput{Username: name}
	if secret := strings.TrimSpace(r.PostFormValue("secret")); secret != "" {
		if !secretRe.MatchString(secret) {
			http.Error(w, "секрет: ровно 32 hex-символа в нижнем регистре", http.StatusBadRequest)
			return
		}
		in.Secret = secret
	}
	var err error
	if in.MaxUniqueIPs, err = parseUintPtr(r.PostFormValue("max_unique_ips")); err != nil {
		http.Error(w, "лимит IP: нужно целое число", http.StatusBadRequest)
		return
	}
	if in.MaxTCPConns, err = parseUintPtr(r.PostFormValue("max_tcp_conns")); err != nil {
		http.Error(w, "лимит соединений: нужно целое число", http.StatusBadRequest)
		return
	}
	if _, err := s.telemt.CreateUser(r.Context(), in); err != nil {
		s.telemtError(w, r, err)
		return
	}
	s.audit(r, "user.create", name, "ok")
	http.Redirect(w, r, "/users/"+name, http.StatusSeeOther)
}

func parseUintPtr(raw string) (*uint64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if !s.isAdmin(r) {
		http.Error(w, "нет прав", http.StatusForbidden)
		return false
	}
	return true
}

func (s *Server) audit(r *http.Request, action, target, result string) {
	sess := s.currentSession(r)
	actor := ""
	if sess != nil {
		actor = sess.User
	}
	addr, _ := clientAddr(r)
	_ = s.store.Audit(r.Context(), store.AuditEvent{
		Actor: actor, Action: action, Target: target, Result: result, IP: addr.String(),
	})
}

func (s *Server) confirmMatches(w http.ResponseWriter, r *http.Request, name string) bool {
	if r.PostFormValue("confirm") != name {
		http.Error(w, "введите имя пользователя для подтверждения", http.StatusBadRequest)
		return false
	}
	return true
}

func (s *Server) rotateSecret(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) || !s.checkCSRF(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if !s.confirmMatches(w, r, name) {
		return
	}
	if err := s.telemt.RotateSecret(r.Context(), name); err != nil {
		s.telemtError(w, r, err)
		return
	}
	s.audit(r, "user.rotate", name, "ok")
	http.Redirect(w, r, "/users/"+name, http.StatusSeeOther)
}

func (s *Server) enableUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) || !s.checkCSRF(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if err := s.telemt.EnableUser(r.Context(), name); err != nil {
		s.telemtError(w, r, err)
		return
	}
	s.audit(r, "user.enable", name, "ok")
	http.Redirect(w, r, "/users/"+name, http.StatusSeeOther)
}

func (s *Server) disableUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) || !s.checkCSRF(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if err := s.telemt.DisableUser(r.Context(), name); err != nil {
		s.telemtError(w, r, err)
		return
	}
	s.audit(r, "user.disable", name, "ok")
	http.Redirect(w, r, "/users/"+name, http.StatusSeeOther)
}

func (s *Server) updateLimits(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) || !s.checkCSRF(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	patch := map[string]any{}
	if v := strings.TrimSpace(r.PostFormValue("max_unique_ips")); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			http.Error(w, "лимит IP: нужно целое число", http.StatusBadRequest)
			return
		}
		patch["max_unique_ips"] = n
	} else if r.PostFormValue("clear_max_unique_ips") == "1" {
		patch["max_unique_ips"] = nil
	}
	if v := strings.TrimSpace(r.PostFormValue("max_tcp_conns")); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			http.Error(w, "лимит соединений: нужно целое число", http.StatusBadRequest)
			return
		}
		patch["max_tcp_conns"] = n
	}
	if len(patch) == 0 {
		http.Redirect(w, r, "/users/"+name, http.StatusSeeOther)
		return
	}
	if err := s.telemt.PatchUser(r.Context(), name, patch); err != nil {
		s.telemtError(w, r, err)
		return
	}
	s.audit(r, "user.limits", name, "ok")
	http.Redirect(w, r, "/users/"+name, http.StatusSeeOther)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) || !s.checkCSRF(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if !s.confirmMatches(w, r, name) {
		return
	}
	users, err := s.telemt.ListUsers(r.Context())
	if err != nil {
		s.telemtError(w, r, err)
		return
	}
	if len(users) <= 1 {
		http.Error(w, "нельзя удалить последнего пользователя", http.StatusConflict)
		return
	}
	if err := s.telemt.DeleteUser(r.Context(), name); err != nil {
		s.telemtError(w, r, err)
		return
	}
	s.audit(r, "user.delete", name, "ok")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) renameUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) || !s.checkCSRF(w, r) {
		return
	}
	oldName := chi.URLParam(r, "name")
	newName := strings.TrimSpace(r.PostFormValue("new_name"))
	if !nameRe.MatchString(newName) {
		http.Error(w, "новое имя: латиница, цифры, точка, дефис, подчёркивание, до 64 символов", http.StatusBadRequest)
		return
	}
	if err := s.telemt.Rename(r.Context(), oldName, newName); err != nil {
		var partial *telemt.RenamePartialError
		if errors.As(err, &partial) {
			s.audit(r, "user.rename", oldName, "partial")
			http.Error(w, "новый пользователь создан, старый удалить не удалось. Оба ключа работают, повторите удаление старого вручную.",
				http.StatusConflict)
			return
		}
		s.audit(r, "user.rename", oldName, "error")
		s.telemtError(w, r, err)
		return
	}
	s.audit(r, "user.rename", oldName+" -> "+newName, "ok")
	http.Redirect(w, r, "/users/"+newName, http.StatusSeeOther)
}
