package web

import (
	"net/http"
	"strings"

	"github.com/McConfig/telemt-admin/internal/store"
)

const principalCtxKey ctxKey = 2

// principal — единый субъект запроса: пользователь UI или сервисный аккаунт.
type principal struct {
	Name    string
	Kind    string // session или service
	Scope   string
	TokenID string
	Role    store.Role
	Session *store.Session
}

func (s *Server) currentPrincipal(r *http.Request) *principal {
	p, _ := r.Context().Value(principalCtxKey).(*principal)
	return p
}

// sessionUserName возвращает имя только для cookie-сессии; сервисный
// токен не является пользователем UI и не может ссылаться на себя.
func (s *Server) sessionUserName(r *http.Request) string {
	if p := s.currentPrincipal(r); p != nil && p.Kind == "session" {
		return p.Name
	}
	return ""
}

func (s *Server) actor(r *http.Request) string {
	if p := s.currentPrincipal(r); p != nil {
		return p.Name
	}
	return ""
}

func roleForScope(scope store.ServiceScope) store.Role {
	switch scope {
	case store.ScopeAdmin, store.ScopeWrite:
		return store.RoleAdmin
	default:
		return store.RoleViewer
	}
}

func servicePathAllowed(scope, path string) bool {
	if strings.HasPrefix(path, "/api/services") {
		return false
	}
	if path == "/api/me" || path == "/api/logout" || path == "/api/summary" ||
		path == "/api/users" || strings.HasPrefix(path, "/api/users/") {
		return true
	}
	if scope != string(store.ScopeAdmin) {
		return false
	}
	return path == "/api/audit" || path == "/api/audit.csv" ||
		path == "/api/diagnostics" ||
		strings.HasPrefix(path, "/api/settings")
}

func (s *Server) requireSessionAdmin(w http.ResponseWriter, r *http.Request) bool {
	p := s.currentPrincipal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "требуется вход")
		return false
	}
	if p.Kind != "session" {
		writeError(w, http.StatusForbidden, "session_required",
			"управление сервисными аккаунтами доступно только из интерфейса")
		return false
	}
	if p.Role != store.RoleAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "нет прав")
		return false
	}
	return true
}
