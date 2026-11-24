package store

import (
	"context"
	"testing"
	"time"
)

func TestIPHistoryAndStats(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	t0 := time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)

	geo := map[string]IPGeo{
		"192.0.2.1": {Country: "NL", CountryName: "Нидерланды", ASN: 60781, ASNOrg: "LeaseWeb"},
	}
	if err := st.RecordIPObservation(ctx, "alice", []string{"192.0.2.1", "192.0.2.2"}, geo, 1000, t0); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordIPObservation(ctx, "alice", []string{"192.0.2.1", "192.0.2.3"}, geo, 1500, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	stats, err := st.UserIPStats(ctx, "alice", t0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if stats.Distinct24h != 3 || stats.Distinct7d != 3 || stats.New24h != 3 {
		t.Fatalf("агрегаты: %+v", stats)
	}
	if stats.MaxToday != 2 {
		t.Fatalf("пик за день: %d", stats.MaxToday)
	}
	if stats.TrafficToday != 500 {
		t.Fatalf("трафик за день: %d", stats.TrafficToday)
	}

	history, err := st.IPHistory(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("история: %+v", history)
	}
	if !history[0].LastSeen.Equal(t0.Add(time.Minute)) && !history[0].LastSeen.Equal(t0) {
		t.Fatalf("порядок истории: %+v", history)
	}
	var enriched *IPStat
	for i := range history {
		if history[i].IP == "192.0.2.1" {
			enriched = &history[i]
		}
	}
	if enriched == nil || enriched.Country != "NL" || enriched.CountryName != "Нидерланды" ||
		enriched.ASN != 60781 || enriched.ASNOrg != "LeaseWeb" {
		t.Fatalf("GeoIP в истории: %+v", enriched)
	}

	// Новые сутки: день обнуляется, дельта считается от последнего значения.
	next := t0.Add(24 * time.Hour)
	if err := st.RecordIPObservation(ctx, "alice", []string{"192.0.2.1"}, nil, 1700, next); err != nil {
		t.Fatal(err)
	}
	stats, _ = st.UserIPStats(ctx, "alice", next)
	if stats.TrafficToday != 200 {
		t.Fatalf("трафик новых суток: %d", stats.TrafficToday)
	}
	traffic, err := st.TrafficHistory(ctx, "alice", 7, next)
	if err != nil {
		t.Fatal(err)
	}
	if len(traffic) != 2 {
		t.Fatalf("история трафика: %+v", traffic)
	}
	if traffic[0].Bytes != 500 || traffic[1].Bytes != 200 {
		t.Fatalf("значения трафика: %+v", traffic)
	}
}

func TestRetentionExpiresHistory(t *testing.T) {
	ctx := context.Background()
	st, mr := newTestStore(t)
	st.SetRetention(time.Hour)
	now := time.Now().UTC()
	if err := st.RecordIPObservation(ctx, "bob", []string{"192.0.2.5"}, nil, 100, now); err != nil {
		t.Fatal(err)
	}
	mr.FastForward(2 * time.Hour)
	stats, err := st.UserIPStats(ctx, "bob", now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if stats.Distinct24h != 0 {
		t.Fatalf("история не истекла: %+v", stats)
	}
}

func TestGeoCounters(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	t0 := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

	geo := map[string]IPGeo{
		"192.0.2.10": {Country: "NL", CountryName: "Нидерланды"},
		"192.0.2.11": {Country: "RU", CountryName: "Россия"},
	}
	if err := st.RecordIPObservation(ctx, "alice", []string{"192.0.2.10", "192.0.2.11"}, geo, 100, t0); err != nil {
		t.Fatal(err)
	}
	stats, err := st.UserIPStats(ctx, "alice", t0)
	if err != nil {
		t.Fatal(err)
	}
	if stats.NewCountries24h != 0 {
		t.Fatalf("в первый день новые страны не считаем: %+v", stats)
	}
	if stats.Foreign24h != 1 || stats.Countries24h != 2 || stats.Countries7d != 2 {
		t.Fatalf("страны в первый день: %+v", stats)
	}

	// Через сутки появился новый адрес в Германии.
	t1 := t0.Add(25 * time.Hour)
	geo2 := map[string]IPGeo{
		"192.0.2.20": {Country: "DE", CountryName: "Германия"},
		"192.0.2.11": {Country: "RU", CountryName: "Россия"},
	}
	if err := st.RecordIPObservation(ctx, "alice", []string{"192.0.2.20", "192.0.2.11"}, geo2, 300, t1); err != nil {
		t.Fatal(err)
	}
	stats, err = st.UserIPStats(ctx, "alice", t1)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Foreign24h != 1 {
		t.Fatalf("иностранных за сутки: %+v", stats)
	}
	if stats.NewCountries24h != 1 {
		t.Fatalf("новая страна DE: %+v", stats)
	}
	if stats.Countries7d != 3 {
		t.Fatalf("стран за 7 дней: %+v", stats)
	}

	// Повторная страна новой не считается.
	if err := st.RecordIPObservation(ctx, "alice",
		[]string{"192.0.2.20", "192.0.2.11"}, geo2, 400, t1.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	stats, _ = st.UserIPStats(ctx, "alice", t1.Add(time.Hour))
	if stats.NewCountries24h != 1 {
		t.Fatalf("DE уже была, новых быть не должно: %+v", stats)
	}
}

func TestTrustedSetGet(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	if err := st.SetTrusted(ctx, "alice", Trusted{
		Countries: []string{"TR", "GE"},
		IPs:       []string{"192.0.2.50"},
	}); err != nil {
		t.Fatal(err)
	}
	trusted, err := st.GetTrusted(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(trusted.Countries) != 2 || trusted.Countries[0] != "GE" || trusted.Countries[1] != "TR" {
		t.Fatalf("страны: %+v", trusted)
	}
	if len(trusted.IPs) != 1 || trusted.IPs[0] != "192.0.2.50" {
		t.Fatalf("адреса: %+v", trusted)
	}
	if err := st.SetTrusted(ctx, "alice", Trusted{}); err != nil {
		t.Fatal(err)
	}
	trusted, _ = st.GetTrusted(ctx, "alice")
	if len(trusted.Countries) != 0 || len(trusted.IPs) != 0 {
		t.Fatalf("очистка: %+v", trusted)
	}
}
