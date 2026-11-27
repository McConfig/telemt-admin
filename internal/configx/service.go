package configx

import (
	"time"

	"github.com/McConfig/telemt-admin/internal/settings"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
)

// Service — сборка, проверка и применение конфигурации.
type Service struct {
	st       *store.Store
	settings *settings.Service
	telemt   *telemt.Client
	now      func() time.Time
}

// New создаёт сервис обмена конфигурацией.
func New(st *store.Store, s *settings.Service, tc *telemt.Client, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{st: st, settings: s, telemt: tc, now: now}
}
