package web

import "net/http"

func (s *Server) staticHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, staticFS, "static/"+r.URL.Path[len("/static/"):])
}
