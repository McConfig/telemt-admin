package web

import "net/http"

// Заглушки маршрутов: реализуются в задачах 9–15.
func (s *Server) loginPage(w http.ResponseWriter, r *http.Request)     { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) loginSubmit(w http.ResponseWriter, r *http.Request)   { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) logout(w http.ResponseWriter, r *http.Request)        { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) usersPage(w http.ResponseWriter, r *http.Request)     { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) newUserPage(w http.ResponseWriter, r *http.Request)   { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) createUser(w http.ResponseWriter, r *http.Request)    { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) userPage(w http.ResponseWriter, r *http.Request)      { http.Error(w, "не реализовано", http.StatusNotImplemented) }
func (s *Server) userQR(w http.ResponseWriter, r *http.Request)        { http.Error(w, "не реализовано", http.StatusNotImplemented) }
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
func (s *Server) checkCSRF(w http.ResponseWriter, r *http.Request) bool { return false }
func (s *Server) redirectLogin(w http.ResponseWriter, r *http.Request)  { http.Redirect(w, r, "/login", http.StatusSeeOther) }
func (s *Server) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
}
func (s *Server) staticHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, staticFS, r.URL.Path[len("/static/"):])
}
