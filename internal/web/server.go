package web

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"time"

	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
	"github.com/go-chi/chi/v5"
)

//go:embed all:dist
var distFS embed.FS

// Server — HTTP-слой приложения: JSON API и статика SPA.
type Server struct {
	cfg    *config.Config
	store  *store.Store
	telemt *telemt.Client
	router chi.Router
	assets http.Handler
	now    func() time.Time
}

// New собирает сервер и маршруты.
func New(cfg *config.Config, st *store.Store, tc *telemt.Client, metrics http.Handler) *Server {
	s := &Server{cfg: cfg, store: st, telemt: tc, now: time.Now}
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(fmt.Sprintf("dist: %v", err))
	}
	s.assets = http.FileServerFS(sub)

	r := chi.NewRouter()
	r.Use(s.allowlist)
	r.Use(s.securityHeaders)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	if metrics != nil {
		r.Get("/metrics", metrics.ServeHTTP)
	}
	r.Route("/api", func(r chi.Router) {
		r.Post("/login", s.apiLogin)
		r.Group(func(r chi.Router) {
			r.Use(s.withAPISession)
			r.Get("/me", s.apiMe)
			r.Group(func(r chi.Router) {
				r.Use(s.apiCSRF)
				r.Post("/logout", s.apiLogout)
				r.Get("/users", s.apiUsers)
				r.Post("/users", s.apiCreateUser)
				r.Get("/users/{name}", s.apiUser)
				r.Patch("/users/{name}", s.apiUpdateLimits)
				r.Delete("/users/{name}", s.apiDeleteUser)
				r.Post("/users/{name}/rotate", s.apiRotate)
				r.Post("/users/{name}/enable", s.apiEnable)
				r.Post("/users/{name}/disable", s.apiDisable)
				r.Post("/users/{name}/rename", s.apiRename)
				r.Get("/users/{name}/qr.png", s.apiQR)
				r.Get("/audit", s.apiAudit)
				r.Get("/account/sessions", s.apiSessions)
				r.Post("/account/password", s.apiChangePassword)
				r.Post("/account/sessions/{id}/revoke", s.apiRevokeSession)
			})
		})
	})
	r.Handle("/*", s.spaHandler(sub))
	s.router = r
	return s
}

// Handler возвращает корневой обработчик.
func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) spaHandler(sub fs.FS) http.Handler {
	index, err := fs.ReadFile(sub, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err != nil {
			http.Error(w, "frontend не собран: выполните npm run build в frontend/", http.StatusServiceUnavailable)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		name := r.URL.Path
		if name != "/" {
			if _, err := fs.Stat(sub, name[1:]); err == nil {
				s.assets.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(index)
	})
}

func mustURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}
