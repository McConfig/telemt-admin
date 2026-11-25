package collector

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/settings"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/alicebob/miniredis/v2"
)

func TestCurrentIntervalReadsSettings(t *testing.T) {
	mr := miniredis.RunT(t)
	st, err := store.Open(context.Background(), "redis://"+mr.Addr()+"/0", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	base, _ := url.Parse("https://tgadmin.example.com")
	cfg := &config.Config{
		BaseURL: base, CollectInterval: time.Minute, SessionTTL: time.Hour,
		LinkMode: "tls", RiskIPsWarn: 3, RiskIPsHigh: 5, RiskDistinct7d: 15,
		HomeCountry: "RU", Retention: 720 * time.Hour,
	}
	svc := settings.New(cfg, st)
	if err := svc.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	c := &Collector{interval: time.Minute, svc: svc}
	if got := c.currentInterval(context.Background()); got != time.Minute {
		t.Fatalf("интервал по умолчанию: %s", got)
	}
	if _, _, err := svc.Save(context.Background(), svc.Version(context.Background()),
		map[string]json.RawMessage{"collect.interval": json.RawMessage(`"10s"`)}, nil); err != nil {
		t.Fatal(err)
	}
	if got := c.currentInterval(context.Background()); got != 10*time.Second {
		t.Fatalf("интервал после настройки: %s", got)
	}
}
