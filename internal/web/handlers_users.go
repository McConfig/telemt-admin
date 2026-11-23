package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/McConfig/telemt-admin/internal/telemt"
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
	switch {
	case errors.As(err, &apiErr):
		http.Error(w, "telemt API: "+apiErr.Message, http.StatusBadGateway)
	default:
		http.Error(w, "telemt не отвечает: "+err.Error(), http.StatusBadGateway)
	}
}
