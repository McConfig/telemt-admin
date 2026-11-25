package store

import (
	"context"
	"testing"
	"time"
)

func TestRiskDays(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)

	day1 := time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 11, 25, 11, 0, 0, 0, time.UTC)

	geo := map[string]IPGeo{
		"192.0.2.10": {Country: "RU", CountryName: "Россия", ASN: 25513, ASNOrg: "MGTS"},
		"198.51.100.7": {
			Country: "NL", CountryName: "Нидерланды", ASN: 60781, ASNOrg: "LeaseWeb",
		},
	}
	if err := st.RecordIPObservation(ctx, "alice", []string{"192.0.2.10"}, geo, 1000, day1); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordIPObservation(ctx, "alice", []string{"192.0.2.10", "198.51.100.7"}, geo, 2500, day2); err != nil {
		t.Fatal(err)
	}

	days, err := st.RiskDays(ctx, "alice", 2, day2)
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 2 {
		t.Fatalf("дней %d, ожидалось 2", len(days))
	}

	today := days[0]
	if today.Day != "2026-11-25" {
		t.Fatalf("первый день %s", today.Day)
	}
	if today.Distinct != 2 || today.Max != 2 || today.NewIPs != 1 {
		t.Fatalf("агрегаты сегодня: %+v", today)
	}
	if len(today.NewCountries) != 1 || today.NewCountries[0] != "NL" {
		t.Fatalf("новые страны сегодня: %v", today.NewCountries)
	}
	if today.Traffic != 1500 {
		t.Fatalf("трафик сегодня: %d", today.Traffic)
	}
	if len(today.IPs) != 2 {
		t.Fatalf("адресов сегодня: %d", len(today.IPs))
	}
	for _, ip := range today.IPs {
		if ip.DayFirst.IsZero() || ip.DayLast.IsZero() {
			t.Fatalf("нет времени внутри дня: %+v", ip)
		}
		if ip.IP == "198.51.100.7" && !ip.New {
			t.Fatal("новый адрес не помечен")
		}
		if ip.IP == "192.0.2.10" && ip.New {
			t.Fatal("старый адрес помечен новым")
		}
	}

	yesterday := days[1]
	if yesterday.Day != "2026-11-24" || yesterday.Distinct != 1 || yesterday.NewIPs != 1 {
		t.Fatalf("вчера: %+v", yesterday)
	}
	if len(yesterday.IPs) != 1 || !yesterday.IPs[0].New {
		t.Fatalf("адреса вчера: %+v", yesterday.IPs)
	}
}

func TestRiskDaysEmpty(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	days, err := st.RiskDays(ctx, "nobody", 7, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 0 {
		t.Fatalf("ожидался пустой отчёт, получено %d", len(days))
	}
}
