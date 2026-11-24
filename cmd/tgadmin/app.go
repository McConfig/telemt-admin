package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/McConfig/telemt-admin/internal/collector"
	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/geoip"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
	"github.com/McConfig/telemt-admin/internal/web"
)

// newHandler собирает рабочее приложение: Valkey, клиент telemt, HTTP-слой.
func newHandler(ctx context.Context, cfg *config.Config) (http.Handler, func() error, error) {
	st, err := store.Open(ctx, cfg.ValkeyURL, cfg.ValkeyPassword)
	if err != nil {
		return nil, nil, err
	}
	st.SetRetention(cfg.Retention)
	tc := telemt.New(cfg.TelemtURL, cfg.TelemtToken, 15*time.Second)
	geo, err := geoip.Open(cfg.GeoIPCity, cfg.GeoIPASN)
	if err != nil {
		slog.Warn("GeoIP выключен", "err", err)
	}
	if geo != nil && geo.Enabled() {
		slog.Info("GeoIP включён", "city", cfg.GeoIPCity, "asn", cfg.GeoIPASN)
	} else {
		slog.Warn("базы GeoIP не найдены, страна и оператор собираться не будут",
			"city", cfg.GeoIPCity, "asn", cfg.GeoIPASN)
	}
	col := collector.New(st, tc, geo, cfg.CollectInterval,
		int64(cfg.RiskIPsWarn), int64(cfg.RiskIPsHigh), int64(cfg.RiskDistinct7d))
	go col.Run(ctx)
	metrics := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		col.ServeHTTP(w, r)
		web.WriteHTTPMetrics(w)
	})
	closeFn := func() error {
		geo.Close()
		return st.Close()
	}
	return web.New(cfg, st, tc, metrics).Handler(), closeFn, nil
}
