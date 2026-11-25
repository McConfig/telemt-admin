package web

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/go-chi/chi/v5"
)

type adminUser struct {
	Name              string `json:"name"`
	Role              string `json:"role"`
	Disabled          bool   `json:"disabled"`
	CreatedAt         string `json:"createdAt"`
	PasswordChangedAt string `json:"passwordChangedAt"`
	Sessions          int64  `json:"sessions"`
	TOTPEnabled       bool   `json:"totpEnabled"`
}

func (s *Server) adminView(r *http.Request, u store.User) adminUser {
	sessions, _ := s.store.CountSessions(r.Context(), u.Name)
	return adminUser{
		Name: u.Name, Role: string(u.Role), Disabled: u.Disabled,
		CreatedAt:         u.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		PasswordChangedAt: u.PasswordChangedAt.UTC().Format("2006-01-02T15:04:05Z"),
		Sessions:          sessions,
		TOTPEnabled:       u.TOTPEnabled,
	}
}

func (s *Server) apiAdmins(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	out := make([]adminUser, 0, len(users))
	for _, u := range users {
		out = append(out, s.adminView(r, u))
	}
	writeJSON(w, http.StatusOK, out)
}

type adminCreateRequest struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

func (s *Server) apiCreateAdmin(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	var in adminCreateRequest
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	name := strings.TrimSpace(in.Name)
	if !nameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "имя: латиница, цифры, точка, дефис, подчёркивание, до 64 символов")
		return
	}
	role := store.Role(in.Role)
	if role != store.RoleAdmin && role != store.RoleViewer {
		writeError(w, http.StatusBadRequest, "bad_request", "роль: admin или viewer")
		return
	}
	password, err := passwordOrDefault(in.Password, s.settings.PasswordMinLength(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	hash, err := auth.Hash(password)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	if err := s.store.CreateUser(r.Context(), store.User{Name: name, Hash: hash, Role: role}); err != nil {
		if err == store.ErrExists {
			writeError(w, http.StatusConflict, "user_exists", "пользователь уже существует")
			return
		}
		s.serverErrorJSON(w, r, err)
		return
	}
	s.audit(r, "admin.create", name, "ok")
	u, _ := s.store.GetUser(r.Context(), name)
	resp := map[string]any{"user": s.adminView(r, *u)}
	if in.Password == "" {
		resp["password"] = password
	}
	writeJSON(w, http.StatusCreated, resp)
}

type adminPatchRequest struct {
	Role     string `json:"role"`
	Disabled *bool  `json:"disabled"`
}

func (s *Server) apiPatchAdmin(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	var in adminPatchRequest
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	u, err := s.store.GetUser(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "пользователь не найден")
		return
	}
	if in.Role != "" {
		role := store.Role(in.Role)
		if role != store.RoleAdmin && role != store.RoleViewer {
			writeError(w, http.StatusBadRequest, "bad_request", "роль: admin или viewer")
			return
		}
		if u.Role == store.RoleAdmin && role != store.RoleAdmin {
			ok, err := s.otherActiveAdmin(r, name)
			if err != nil || !ok {
				writeError(w, http.StatusConflict, "last_admin", "нельзя снять роль с последнего администратора")
				return
			}
		}
		if role != u.Role {
			if err := s.rdbSetRole(r, name, role); err != nil {
				s.serverErrorJSON(w, r, err)
				return
			}
		}
	}
	if in.Disabled != nil {
		if *in.Disabled {
			if name == s.currentSession(r).User {
				writeError(w, http.StatusConflict, "self", "нельзя отключить себя")
				return
			}
			if u.Role == store.RoleAdmin {
				ok, err := s.otherActiveAdmin(r, name)
				if err != nil || !ok {
					writeError(w, http.StatusConflict, "last_admin", "нельзя отключить последнего администратора")
					return
				}
			}
		}
		if err := s.store.SetDisabled(r.Context(), name, *in.Disabled); err != nil {
			s.serverErrorJSON(w, r, err)
			return
		}
		if *in.Disabled {
			_ = s.store.DeleteUserSessions(r.Context(), name, "")
		}
		s.audit(r, "admin.disable", name, "ok")
	}
	u, _ = s.store.GetUser(r.Context(), name)
	writeJSON(w, http.StatusOK, s.adminView(r, *u))
}

func (s *Server) apiResetAdminPassword(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	var in adminCreateRequest
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	password, err := passwordOrDefault(in.Password, s.settings.PasswordMinLength(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	hash, err := auth.Hash(password)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	if err := s.store.SetPassword(r.Context(), name, hash); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "пользователь не найден")
		return
	}
	_ = s.store.DeleteUserSessions(r.Context(), name, "")
	s.audit(r, "admin.password", name, "ok")
	resp := map[string]any{"ok": true}
	if in.Password == "" {
		resp["password"] = password
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) apiDeleteAdmin(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if name == s.currentSession(r).User {
		writeError(w, http.StatusConflict, "self", "нельзя удалить себя")
		return
	}
	u, err := s.store.GetUser(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "пользователь не найден")
		return
	}
	if u.Role == store.RoleAdmin {
		ok, err := s.otherActiveAdmin(r, name)
		if err != nil || !ok {
			writeError(w, http.StatusConflict, "last_admin", "нельзя удалить последнего администратора")
			return
		}
	}
	if err := s.store.DeleteUser(r.Context(), name); err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	s.audit(r, "admin.delete", name, "ok")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiRevokeAdminSessions(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if name == s.currentSession(r).User {
		writeError(w, http.StatusConflict, "self", "используйте выход из аккаунта")
		return
	}
	if err := s.store.DeleteUserSessions(r.Context(), name, ""); err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	s.audit(r, "admin.sessions", name, "ok")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) otherActiveAdmin(r *http.Request, except string) (bool, error) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		return false, err
	}
	for _, u := range users {
		if u.Name != except && u.Role == store.RoleAdmin && !u.Disabled {
			return true, nil
		}
	}
	return false, nil
}

func (s *Server) rdbSetRole(r *http.Request, name string, role store.Role) error {
	return s.store.SetRole(r.Context(), name, role)
}

func passwordOrDefault(raw string, min int) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		buf := make([]byte, 12)
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		return base64.RawURLEncoding.EncodeToString(buf), nil
	}
	if err := auth.CheckPolicy(raw, min); err != nil {
		return "", err
	}
	return raw, nil
}
