package web

import (
	"net/http"
	"path"
	"strings"
)

func (s *Server) staticHandler(w http.ResponseWriter, r *http.Request) {
	name := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/static/"))
	if strings.HasPrefix(name, "/..") || name == "/" {
		http.NotFound(w, r)
		return
	}
	// #nosec G703 -- имя очищено, файлы берутся из встроенного embed.FS.
	http.ServeFileFS(w, r, staticFS, "static"+name)
}
