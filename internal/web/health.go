package web

import (
	"context"
	"net/http"
	"time"
)

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if _, err := s.store.CountUsers(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "store", "Valkey недоступен")
		return
	}
	if err := s.telemt.Ready(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "telemt", "telemt API недоступен")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
