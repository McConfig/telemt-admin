package web

import "net/http"

// Заглушки маршрутов: реализуются в задачах 9–15.
func (s *Server) newUserPage(w http.ResponseWriter, r *http.Request)   { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) createUser(w http.ResponseWriter, r *http.Request)    { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) rotateSecret(w http.ResponseWriter, r *http.Request)  { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) enableUser(w http.ResponseWriter, r *http.Request)    { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) disableUser(w http.ResponseWriter, r *http.Request)   { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) renameUser(w http.ResponseWriter, r *http.Request)    { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) updateLimits(w http.ResponseWriter, r *http.Request)  { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request)    { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) auditPage(w http.ResponseWriter, r *http.Request)     { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) accountPage(w http.ResponseWriter, r *http.Request)   { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) revokeSession(w http.ResponseWriter, r *http.Request) { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) staticHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, staticFS, "static/"+r.URL.Path[len("/static/"):])
}
