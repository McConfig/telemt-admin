//go:build demo

// Command demo-seed наполняет демо-стенд.
//
// Создаёт пользователей интерфейса, пишет историю адресов и стран за семь
// дней, добавляет доверенные страны и записи аудита. Клиентов прокси
// создаёт fake-telemt, поэтому seed сначала читает их список из Control API.
//
// Запуск в compose:
//
//	docker compose --profile seed run --rm seed
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
)

const historyDays = 8

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	st, err := store.Open(ctx, env("VALKEY_URL", "redis://valkey:6379/0"), env("VALKEY_PASSWORD", "demo-valkey-password"))
	if err != nil {
		log.Fatalf("Valkey: %v", err)
	}
	defer func() { _ = st.Close() }()

	tc := telemt.New(env("TELEMT_API_URL", "http://fake-telemt:9091"), env("TELEMT_API_TOKEN", "demo-token"), 10*time.Second)
	users, err := tc.ListUsers(ctx)
	if err != nil {
		log.Fatalf("Control API: %v (запущен ли fake-telemt?)", err)
	}

	if err := ensureUIUser(ctx, st, "demo", env("DEMO_ADMIN_PASSWORD", "demo-admin-password"), store.RoleAdmin); err != nil {
		log.Fatalf("admin: %v", err)
	}
	if err := ensureUIUser(ctx, st, "viewer", env("DEMO_VIEWER_PASSWORD", "demo-viewer-password"), store.RoleViewer); err != nil {
		log.Fatalf("viewer: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Minute)
	for _, u := range users {
		if err := seedHistory(ctx, st, u, now); err != nil {
			log.Fatalf("история %s: %v", u.Username, err)
		}
	}
	if err := seedTrusted(ctx, st); err != nil {
		log.Fatalf("доверенные: %v", err)
	}
	if err := seedAudit(ctx, st, now); err != nil {
		log.Fatalf("аудит: %v", err)
	}

	fmt.Printf("демо готово: %d клиентов, пользователи demo/viewer\n", len(users))
	fmt.Println("вход: http://localhost:8090")
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func ensureUIUser(ctx context.Context, st *store.Store, name, password string, role store.Role) error {
	hash, err := auth.Hash(password)
	if err != nil {
		return err
	}
	err = st.CreateUser(ctx, store.User{Name: name, Hash: hash, Role: role})
	if errors.Is(err, store.ErrExists) {
		if err := st.SetPassword(ctx, name, hash); err != nil {
			return err
		}
		if err := st.SetRole(ctx, name, role); err != nil {
			return err
		}
		return st.SetDisabled(ctx, name, false)
	}
	return err
}

// seedHistory пишет наблюдения за восемь дней: сегодня и семь предыдущих.
func seedHistory(ctx context.Context, st *store.Store, u telemt.User, now time.Time) error {
	if err := st.DeleteIPHistory(ctx, u.Username); err != nil {
		return err
	}
	base := u.ActiveUniqueIPList
	if len(base) == 0 {
		base = u.RecentUniqueIPList
	}
	if len(base) == 0 {
		return nil
	}

	for i := 0; i < historyDays; i++ {
		day := now.AddDate(0, 0, -(historyDays - 1 - i))
		if i < historyDays-1 {
			day = time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, time.UTC)
		}
		ips := ipsForDay(u.Username, base, i)
		geo := map[string]store.IPGeo{}
		for _, ip := range ips {
			geo[ip] = geoFor(ip)
		}
		total := u.TotalOctets * uint64(i+1) / historyDays
		if err := st.RecordIPObservation(ctx, u.Username, ips, geo, total, day); err != nil {
			return err
		}
	}

	current := ipsForDay(u.Username, base, historyDays-1)
	var foreign int64
	codes := map[string]string{}
	operators := map[string]bool{}
	for _, ip := range current {
		g := geoFor(ip)
		if g.Internal {
			continue
		}
		if g.Country != "" && g.Country != "RU" {
			foreign++
		}
		if g.Country != "" {
			codes[g.Country] = g.CountryName
		}
		if g.ASNOrg != "" {
			operators[g.ASNOrg] = true
		}
	}
	codeList := make([]string, 0, len(codes))
	nameList := make([]string, 0, len(codes))
	for code, name := range codes {
		codeList = append(codeList, code)
		nameList = append(nameList, name)
	}
	opList := make([]string, 0, len(operators))
	for op := range operators {
		opList = append(opList, op)
	}
	return st.SaveGeoStats(ctx, u.Username, foreign, codeList, nameList, opList)
}

// ipsForDay добавляет адреса так, чтобы в истории были видны изменения.
func ipsForDay(name string, base []string, day int) []string {
	out := append([]string(nil), base...)
	switch name {
	case "router-home":
		if day >= 3 {
			out = append(out, "192.0.2.77")
		}
	case "travel-phone":
		// Германия появляется только сегодня: так в демо видно новую страну.
		if day < historyDays-1 {
			out = []string{"198.51.100.21"}
		}
	case "phone-main":
		if day >= 5 {
			out = append(out, "198.51.100.7")
		}
	}
	return out
}

func geoFor(ip string) store.IPGeo {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return store.IPGeo{}
	}
	if addr.IsPrivate() || addr.IsLoopback() {
		return store.IPGeo{Internal: true}
	}
	if !addr.Is4() {
		return store.IPGeo{}
	}
	switch {
	case addr.As4()[0] == 192 && addr.As4()[1] == 0 && addr.As4()[2] == 2:
		return store.IPGeo{Country: "RU", CountryName: "Россия", ASN: 25513, ASNOrg: "MGTS"}
	case addr.As4()[0] == 198 && addr.As4()[1] == 51 && addr.As4()[2] == 100:
		return store.IPGeo{Country: "NL", CountryName: "Нидерланды", ASN: 60781, ASNOrg: "LeaseWeb"}
	case addr.As4()[0] == 203 && addr.As4()[1] == 0 && addr.As4()[2] == 113:
		return store.IPGeo{Country: "DE", CountryName: "Германия", ASN: 24940, ASNOrg: "Hetzner"}
	}
	return store.IPGeo{}
}

func seedTrusted(ctx context.Context, st *store.Store) error {
	if err := st.SetTrusted(ctx, "phone-main", store.Trusted{
		Countries: []string{"NL"},
		IPs:       []string{"198.51.100.7"},
	}); err != nil {
		return err
	}
	return st.SetTrusted(ctx, "travel-phone", store.Trusted{Countries: []string{"NL"}})
}

func seedAudit(ctx context.Context, st *store.Store, now time.Time) error {
	events := []store.AuditEvent{
		{At: now.Add(-26 * time.Hour), Actor: "demo", Action: "login", Target: "demo", Result: "ok", IP: "192.0.2.101"},
		{At: now.Add(-25 * time.Hour), Actor: "demo", Action: "user.create", Target: "phone-main", Result: "ok", IP: "192.0.2.101"},
		{At: now.Add(-24 * time.Hour), Actor: "demo", Action: "user.limits", Target: "phone-main", Result: "ok", IP: "192.0.2.101"},
		{At: now.Add(-23 * time.Hour), Actor: "demo", Action: "user.trusted", Target: "phone-main", Result: "ok", IP: "192.0.2.101"},
		{At: now.Add(-12 * time.Hour), Actor: "viewer", Action: "login", Target: "viewer", Result: "ok", IP: "192.0.2.102"},
		{At: now.Add(-11 * time.Hour), Actor: "viewer", Action: "logout", Target: "viewer", Result: "ok", IP: "192.0.2.102"},
		{At: now.Add(-6 * time.Hour), Actor: "demo", Action: "user.rotate", Target: "travel-phone", Result: "ok", IP: "192.0.2.101"},
		{At: now.Add(-5 * time.Hour), Actor: "demo", Action: "user.limits", Target: "tablet-kids", Result: "ok", IP: "192.0.2.101"},
		{At: now.Add(-3 * time.Hour), Actor: "demo", Action: "user.reset_quota", Target: "tablet-kids", Result: "ok", IP: "192.0.2.101"},
		{At: now.Add(-2 * time.Hour), Actor: "demo", Action: "user.rotate", Target: "phone-main", Result: "ok", IP: "192.0.2.101"},
		{At: now.Add(-90 * time.Minute), Actor: "demo", Action: "user.trusted", Target: "travel-phone", Result: "ok", IP: "192.0.2.101"},
		{At: now.Add(-45 * time.Minute), Actor: "demo", Action: "admin.create", Target: "viewer", Result: "ok", IP: "192.0.2.101"},
		{At: now.Add(-30 * time.Minute), Actor: "demo", Action: "user.disable", Target: "test-device", Result: "ok", IP: "192.0.2.101"},
	}
	for _, e := range events {
		if err := st.Audit(ctx, e); err != nil {
			return err
		}
	}
	return nil
}
