package main

import (
	"context"
	"net/http"
	"time"

	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
	"github.com/McConfig/telemt-admin/internal/web"
)

// newHandler собирает рабочее приложение: Valkey, клиент telemt, HTTP-слой.
func newHandler(cfg *config.Config) (http.Handler, func() error, error) {
	st, err := store.Open(context.Background(), cfg.ValkeyURL, cfg.ValkeyPassword)
	if err != nil {
		return nil, nil, err
	}
	tc := telemt.New(cfg.TelemtURL, cfg.TelemtToken, 15*time.Second)
	return web.New(cfg, st, tc).Handler(), st.Close, nil
}
