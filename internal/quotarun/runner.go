// Package quotarun выполняет плановый сброс квот.
package quotarun

import (
	"context"
	"log/slog"
	"time"

	"github.com/McConfig/telemt-admin/internal/settings"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
)

// Runner проверяет расписания и сбрасывает квоты.
type Runner struct {
	st       *store.Store
	tc       *telemt.Client
	svc      *settings.Service
	interval time.Duration
	now      func() time.Time
}

// New создаёт планировщик.
func New(st *store.Store, tc *telemt.Client, svc *settings.Service) *Runner {
	return &Runner{st: st, tc: tc, svc: svc, interval: 30 * time.Second, now: time.Now}
}

// Run проверяет расписания до отмены контекста.
func (r *Runner) Run(ctx context.Context) {
	r.Tick(ctx)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.Tick(ctx)
		}
	}
}

// Tick выполняет один проход. Вызывается таймером и тестами.
func (r *Runner) Tick(ctx context.Context) {
	globalEnabled, globalDays, globalTime := r.svc.QuotaResetPolicy(ctx)
	users, err := r.tc.ListUsers(ctx)
	if err != nil {
		slog.Warn("плановый сброс квот: список клиентов", "err", err)
		return
	}
	now := r.now().UTC()
	for _, u := range users {
		override, _, err := r.st.GetQuotaReset(ctx, u.Username)
		if err != nil {
			slog.Warn("плановый сброс квот: расписание", "user", u.Username, "err", err)
			continue
		}
		enabled, days, at := settings.EffectiveQuotaReset(
			globalEnabled, globalDays, globalTime, override,
		)
		if !enabled {
			continue
		}
		last, ok, err := r.st.QuotaLastRun(ctx, u.Username)
		if err != nil {
			slog.Warn("плановый сброс квот: последний запуск", "user", u.Username, "err", err)
			continue
		}
		if !ok {
			// Первый запуск только задаёт точку отсчёта.
			_ = r.st.SetQuotaLastRun(ctx, u.Username, now)
			continue
		}
		if !settings.QuotaResetDue(last, days, at, now) {
			continue
		}
		locked, err := r.st.TryQuotaLock(ctx, u.Username, 30*time.Second)
		if err != nil {
			slog.Warn("плановый сброс квот: блокировка", "user", u.Username, "err", err)
			continue
		}
		if !locked {
			// Ручной сброс уже идёт: не дублируем и проверим на следующем тике.
			continue
		}
		err = r.tc.ResetQuota(ctx, u.Username)
		r.st.UnlockQuota(ctx, u.Username)
		result := "ok"
		if err != nil {
			result = "error"
			slog.Warn("плановый сброс квоты", "user", u.Username, "err", err)
		}
		_ = r.st.Audit(ctx, store.AuditEvent{
			At: now, Actor: "scheduler", Action: "quota.reset.scheduled",
			Target: u.Username, Result: result,
		})
		// Следующая попытка — через интервал, даже если текущая не удалась.
		_ = r.st.SetQuotaLastRun(ctx, u.Username, now)
	}
}
