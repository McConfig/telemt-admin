package web

import (
	"context"
	"embed"
	"fmt"
	"github.com/McConfig/telemt-admin/internal/collector"
	"io/fs"
	"net/http"
	"net/url"
	"time"

	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/settings"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
	"github.com/go-chi/chi/v5"
)

//go:embed all:dist
var distFS embed.FS

// Server — HTTP-слой приложения: JSON API и статика SPA.
type Server struct {
	cfg       *config.Config
	store     *store.Store
	telemt    *telemt.Client
	settings  *settings.Service
	notifier  Notifier
	router    chi.Router
	assets    http.Handler
	now       func() time.Time
	startedAt time.Time
	status    func() collector.Status
}

// Notifier отправляет события аудита во внешнюю систему.
type Notifier interface {
	Emit(ctx context.Context, event string, payload map[string]any)
	SendNow(ctx context.Context, event string, payload map[string]any) (int, error)
}

// SetNotifier подключает отправитель вебхуков.
func (s *Server) SetNotifier(n Notifier) {
	s.notifier = n
}

// SetCollectorStatus подключает источник состояния сборщика.
func (s *Server) SetCollectorStatus(fn func() collector.Status) {
	s.status = fn
}

// New собирает сервер и маршруты.
func New(cfg *config.Config, st *store.Store, tc *telemt.Client, metrics http.Handler) *Server {
	s := &Server{cfg: cfg, store: st, telemt: tc, settings: settings.New(cfg, st), now: time.Now, startedAt: time.Now()}
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(fmt.Sprintf("dist: %v", err))
	}
	s.assets = http.FileServerFS(sub)

	r := chi.NewRouter()
	r.Use(s.allowlist)
	r.Use(s.securityHeaders)
	r.Use(s.logRequests)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	r.Get("/readyz", s.readyz)
	if metrics != nil {
		r.Get("/metrics", metrics.ServeHTTP)
	}
	r.Route("/api", func(r chi.Router) {
		r.Get("/meta", s.apiMeta)
		r.Post("/login", s.apiLogin)
		r.Post("/login/totp", s.apiLoginTOTP)
		r.Group(func(r chi.Router) {
			r.Use(s.withAPISession)
			r.Get("/me", s.apiMe)
			r.Group(func(r chi.Router) {
				r.Use(s.apiCSRF)
				r.Post("/logout", s.apiLogout)
				r.Get("/users", s.apiUsers)
				r.Post("/users", s.apiCreateUser)
				r.Post("/users/reset-quota", s.apiBulkResetQuota)
				r.Get("/users/{name}", s.apiUser)
				r.Patch("/users/{name}", s.apiUpdateLimits)
				r.Delete("/users/{name}", s.apiDeleteUser)
				r.Post("/users/{name}/rotate", s.apiRotate)
				r.Post("/users/{name}/enable", s.apiEnable)
				r.Post("/users/{name}/disable", s.apiDisable)
				r.Post("/users/{name}/rename", s.apiRename)
				r.Get("/users/{name}/qr.png", s.apiQR)
				r.Post("/users/{name}/reset-quota", s.apiResetQuota)
				r.Get("/users/{name}/history.csv", s.apiHistoryCSV)
				r.Get("/users/{name}/quota-reset", s.apiGetQuotaReset)
				r.Put("/users/{name}/quota-reset", s.apiSetQuotaReset)
				r.Get("/users/{name}/risk-report", s.apiRiskReport)
				r.Get("/users/{name}/risk.csv", s.apiRiskCSV)
				r.Get("/settings", s.apiGetSettings)
				r.Put("/settings", s.apiUpdateSettings)
				r.Get("/settings/templates", s.apiListTemplates)
				r.Post("/settings/templates", s.apiSaveTemplate)
				r.Put("/settings/templates/default", s.apiSetDefaultTemplate)
				r.Post("/settings/webhook/test", s.apiWebhookTest)
				r.Put("/settings/templates/{name}", s.apiSaveTemplate)
				r.Delete("/settings/templates/{name}", s.apiDeleteTemplate)
				r.Get("/settings/backups", s.apiBackups)
				r.Post("/settings/backups/snapshot", s.apiBackupSnapshot)
				r.Get("/settings/system", s.apiSystem)
				r.Get("/settings/history", s.apiSettingsHistory)
				r.Get("/diagnostics", s.apiDiagnostics)
				r.Get("/users/{name}/traffic", s.apiUserTraffic)
				r.Get("/users/{name}/timeline", s.apiUserTimeline)
				r.Get("/summary", s.apiSummary)
				r.Delete("/users/{name}/history", s.apiDeleteHistory)
				r.Get("/users/{name}/trusted", s.apiGetTrusted)
				r.Put("/users/{name}/trusted", s.apiSetTrusted)
				r.Put("/users/{name}/risk", s.apiSetRisk)
				r.Get("/users/{name}/note", s.apiGetNote)
				r.Put("/users/{name}/note", s.apiSetNote)
				r.Get("/admins", s.apiAdmins)
				r.Post("/admins", s.apiCreateAdmin)
				r.Patch("/admins/{name}", s.apiPatchAdmin)
				r.Delete("/admins/{name}", s.apiDeleteAdmin)
				r.Post("/admins/{name}/password", s.apiResetAdminPassword)
				r.Post("/admins/{name}/sessions/revoke", s.apiRevokeAdminSessions)
				r.Get("/audit.csv", s.apiAuditCSV)
				r.Get("/audit", s.apiAudit)
				r.Get("/account/sessions", s.apiSessions)
				r.Post("/account/password", s.apiChangePassword)
				r.Post("/account/sessions/{id}/revoke", s.apiRevokeSession)
				r.Get("/account/totp", s.apiTOTPStatus)
				r.Post("/account/totp/setup", s.apiTOTPSetup)
				r.Post("/account/totp/enable", s.apiTOTPEnable)
				r.Post("/account/totp/disable", s.apiTOTPDisable)
				r.Post("/account/totp/recovery", s.apiTOTPRecovery)
				r.Post("/admins/{name}/totp-reset", s.apiResetAdminTOTP)
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
