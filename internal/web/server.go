package web

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"time"

	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Server — HTTP-слой приложения.
type Server struct {
	cfg    *config.Config
	store  *store.Store
	telemt *telemt.Client
	tmpl   *template.Template
	router chi.Router
	now    func() time.Time
}

// New собирает сервер и маршруты.
func New(cfg *config.Config, st *store.Store, tc *telemt.Client) *Server {
	s := &Server{cfg: cfg, store: st, telemt: tc, now: time.Now}
	s.tmpl = template.Must(template.New("").Funcs(template.FuncMap{
		"fmtTime": func(t time.Time) string { return t.Local().Format("02.01.2006 15:04") },
		"bytes": func(n uint64) string {
			const unit = 1024
			if n < unit {
				return fmt.Sprintf("%d Б", n)
			}
			div, exp := uint64(unit), 0
			for x := n / unit; x >= unit; x /= unit {
				div *= unit
				exp++
			}
			return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
		},
	}).ParseFS(templateFS, "templates/*.html"))

	r := chi.NewRouter()
	r.Use(s.allowlist)
	r.Use(s.securityHeaders)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	r.Get("/static/*", s.staticHandler)
	r.Get("/login", s.loginPage)
	r.Post("/login", s.loginSubmit)

	r.Group(func(r chi.Router) {
		r.Use(s.withSession)
		r.Get("/", s.usersPage)
		r.Get("/users/new", s.newUserPage)
		r.Post("/users", s.createUser)
		r.Get("/users/{name}", s.userPage)
		r.Get("/users/{name}/qr", s.userQR)
		r.Post("/users/{name}/rotate", s.rotateSecret)
		r.Post("/users/{name}/enable", s.enableUser)
		r.Post("/users/{name}/disable", s.disableUser)
		r.Post("/users/{name}/rename", s.renameUser)
		r.Post("/users/{name}/limits", s.updateLimits)
		r.Post("/users/{name}/delete", s.deleteUser)
		r.Get("/audit", s.auditPage)
		r.Get("/account", s.accountPage)
		r.Post("/account/password", s.changePassword)
		r.Post("/account/sessions/{id}/revoke", s.revokeSession)
		r.Post("/logout", s.logout)
	})
	s.router = r
	return s
}

// Handler возвращает корневой обработчик.
func (s *Server) Handler() http.Handler { return s.router }

func mustURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}
