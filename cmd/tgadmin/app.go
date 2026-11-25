package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/McConfig/telemt-admin/internal/collector"
	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/geoip"
	"github.com/McConfig/telemt-admin/internal/logging"
	"github.com/McConfig/telemt-admin/internal/settings"
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
	svc := settings.New(cfg, st)
	if err := svc.Load(ctx); err != nil {
		slog.Warn("настройки из Valkey не прочитаны, используются значения env", "err", err)
	}
	st.SetRetention(svc.Retention(ctx))
	st.SetHomeCountry(svc.HomeCountry(ctx))
	maxFailures, loginWindow := svc.LoginPolicy(ctx)
	st.SetLoginPolicy(maxFailures, loginWindow)
	st.SetAuditMax(svc.AuditMaxEvents(ctx))
	_ = logging.Set(svc.LogLevel(ctx))
	_, _, _ = svc.Require2FA(ctx)
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
	col := collector.New(st, tc, geo, svc.CollectInterval(ctx),
		int64(cfg.RiskIPsWarn), int64(cfg.RiskIPsHigh), int64(cfg.RiskDistinct7d))
	col.SetSettings(svc)
	go col.Run(ctx)
	metrics := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		col.ServeHTTP(w, r)
		web.WriteHTTPMetrics(w)
	})
	closeFn := func() error {
		geo.Close()
		return st.Close()
	}
	handler := web.New(cfg, st, tc, metrics)
	handler.SetCollectorStatus(col.Status)
	return handler.Handler(), closeFn, nil
}
