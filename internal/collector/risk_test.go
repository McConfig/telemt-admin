package collector

import (
	"testing"

	"github.com/McConfig/telemt-admin/internal/store"
)

func TestRiskDefaultAndOverrides(t *testing.T) {
	c := &Collector{warn: 3, high: 5, distinct7d: 15}

	if got := c.risk(Snapshot{Current: 4}, store.Risk{}); got != 1 {
		t.Fatalf("предупреждение по умолчанию: %d", got)
	}
	if got := c.risk(Snapshot{Current: 6}, store.Risk{}); got != 2 {
		t.Fatalf("высокий риск по умолчанию: %d", got)
	}
	if got := c.risk(Snapshot{Current: 1, Distinct7d: 16}, store.Risk{}); got != 1 {
		t.Fatalf("много адресов за 7 дней: %d", got)
	}
	if got := c.risk(Snapshot{Current: 1, NewCountries24h: 1}, store.Risk{}); got != 1 {
		t.Fatalf("новая страна по умолчанию: %d", got)
	}

	warn, high, distinct := 10, 20, 30
	no := false
	cfg := store.Risk{WarnIPs: &warn, HighIPs: &high, Distinct7d: &distinct, NewCountries: &no}

	if got := c.risk(Snapshot{Current: 8, NewCountries24h: 2}, cfg); got != 0 {
		t.Fatalf("переопределение: %d, ожидалось 0", got)
	}
	if got := c.risk(Snapshot{Current: 11}, cfg); got != 1 {
		t.Fatalf("warn переопределён: %d", got)
	}
	if got := c.risk(Snapshot{Current: 21}, cfg); got != 2 {
		t.Fatalf("high переопределён: %d", got)
	}
	if got := c.risk(Snapshot{Current: 1, Distinct7d: 31}, cfg); got != 1 {
		t.Fatalf("distinct переопределён: %d", got)
	}

	// Доверенные адреса не считаются.
	if got := c.risk(Snapshot{Current: 10, TrustedActive: 5}, cfg); got != 0 {
		t.Fatalf("доверенные адреса: %d, ожидалось 0", got)
	}
}
