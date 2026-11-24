package store

import (
	"context"
	"testing"
	"time"
)

func TestUsersAndSessionsCoverage(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	if err := st.CreateUser(ctx, User{Name: "alice", Hash: "h1", Role: RoleViewer}); err != nil {
		t.Fatal(err)
	}
	if n, _ := st.CountUsers(ctx); n != 1 {
		t.Fatalf("пользователей %d", n)
	}
	if err := st.SetRole(ctx, "alice", RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if u, _ := st.GetUser(ctx, "alice"); u.Role != RoleAdmin {
		t.Fatalf("роль %s", u.Role)
	}
	if err := st.SetPassword(ctx, "alice", "h2"); err != nil {
		t.Fatal(err)
	}
	if u, _ := st.GetUser(ctx, "alice"); u.Hash != "h2" {
		t.Fatalf("хеш %s", u.Hash)
	}
	if err := st.SetDisabled(ctx, "alice", true); err != nil {
		t.Fatal(err)
	}
	if u, _ := st.GetUser(ctx, "alice"); !u.Disabled {
		t.Fatal("не отключён")
	}

	tok, err := st.CreateSession(ctx, "alice", "192.0.2.1", "ua", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	tok2, _ := st.CreateSession(ctx, "alice", "192.0.2.2", "ua2", time.Hour)
	if n, _ := st.CountSessions(ctx, "alice"); n != 2 {
		t.Fatalf("сессий %d", n)
	}
	list, err := st.ListSessions(ctx, "alice")
	if err != nil || len(list) != 2 {
		t.Fatalf("список сессий: %+v %v", list, err)
	}
	if err := st.DeleteSessionByID(ctx, "alice", sessionID(tok)); err != nil {
		t.Fatal(err)
	}
	if n, _ := st.CountSessions(ctx, "alice"); n != 1 {
		t.Fatalf("после удаления %d", n)
	}
	if err := st.DeleteUserSessions(ctx, "alice", tok2); err != nil {
		t.Fatal(err)
	}
	if n, _ := st.CountSessions(ctx, "alice"); n != 1 {
		t.Fatalf("должна остаться одна сессия, %d", n)
	}
	if err := st.DeleteUser(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetPassword(ctx, "missing", "x"); err != ErrNotFound {
		t.Fatalf("ожидали ErrNotFound, %v", err)
	}
}

func TestNotesAndIPStatsCoverage(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)

	if err := st.SetNote(ctx, "alice", "телефон"); err != nil {
		t.Fatal(err)
	}
	if note, _ := st.Note(ctx, "alice"); note != "телефон" {
		t.Fatalf("заметка %q", note)
	}
	if err := st.SetNote(ctx, "alice", ""); err != nil {
		t.Fatal(err)
	}
	if note, _ := st.Note(ctx, "alice"); note != "" {
		t.Fatalf("заметка не очищена: %q", note)
	}

	geo := map[string]IPGeo{"192.0.2.10": {Country: "NL", CountryName: "Нидерланды", ASN: 1, ASNOrg: "Org"}}
	if err := st.RecordIPObservation(ctx, "alice", []string{"192.0.2.10"}, geo, 1000, now); err != nil {
		t.Fatal(err)
	}
	if err := st.SetIPGeo(ctx, "alice", "192.0.2.11", IPGeo{Country: "TR", CountryName: "Турция"}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := st.GeoStats(ctx, "alice"); err != nil || ok {
		t.Fatalf("GeoStats до сохранения: %v %v", ok, err)
	}
	if err := st.SaveGeoStats(ctx, "alice", 2, []string{"NL", "TR"}, []string{"Нидерланды", "Турция"}); err != nil {
		t.Fatal(err)
	}
	stats, _, _ := st.GeoStats(ctx, "alice")
	if stats.ForeignCurrent != 2 || len(stats.Countries) != 2 {
		t.Fatalf("SaveGeoStats: %+v", stats)
	}

	if err := st.RecordIPObservation(ctx, "alice", []string{"192.0.2.12"}, geo, 1500, now.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	timeline, err := st.Timeline(ctx, "alice", 7, now.Add(24*time.Hour))
	if err != nil || len(timeline) == 0 {
		t.Fatalf("Timeline: %+v %v", timeline, err)
	}
	if last, ok := st.LastSeen(ctx, "alice"); !ok || last.IsZero() {
		t.Fatalf("LastSeen: %v %v", last, ok)
	}
	if err := st.DeleteIPHistory(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if _, ok := st.LastSeen(ctx, "alice"); ok {
		t.Fatal("история не удалена")
	}
	if _, ok, _ := st.GeoStats(ctx, "alice"); ok {
		t.Fatal("GeoStats не удалены")
	}
}

func TestTOTPStoreCoverage(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	if err := st.CreateUser(ctx, User{Name: "alice", Hash: "h", Role: RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetTOTPPending(ctx, "alice", "pending"); err != nil {
		t.Fatal(err)
	}
	u, _ := st.GetUser(ctx, "alice")
	if u.TOTPPending != "pending" || u.TOTPEnabled {
		t.Fatalf("pending: %+v", u)
	}
	if err := st.EnableTOTP(ctx, "alice", "secret", 42, []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	u, _ = st.GetUser(ctx, "alice")
	if !u.TOTPEnabled || u.TOTPLastCounter != 42 || len(u.RecoveryCodes) != 2 {
		t.Fatalf("после включения: %+v", u)
	}
	if err := st.SetTOTPLastCounter(ctx, "alice", 43); err != nil {
		t.Fatal(err)
	}
	if ok, err := st.ConsumeRecoveryCode(ctx, "alice", "a"); err != nil || !ok {
		t.Fatalf("первый резервный код: %v %v", ok, err)
	}
	if ok, _ := st.ConsumeRecoveryCode(ctx, "alice", "a"); ok {
		t.Fatal("резервный код использован дважды")
	}
	if err := st.SetRecoveryCodes(ctx, "alice", []string{"c"}); err != nil {
		t.Fatal(err)
	}
	if u, _ := st.GetUser(ctx, "alice"); len(u.RecoveryCodes) != 1 {
		t.Fatalf("коды не заменены: %+v", u)
	}
	if err := st.DisableTOTP(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if u, _ := st.GetUser(ctx, "alice"); u.TOTPEnabled || u.RecoveryCodes != nil {
		t.Fatalf("после выключения: %+v", u)
	}

	token, err := st.CreatePendingTOTP(ctx, "alice", "192.0.2.1", "ua", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := st.GetPendingTOTP(ctx, token)
	if err != nil || pending.User != "alice" {
		t.Fatalf("pending TOTP: %+v %v", pending, err)
	}
	if n, _ := st.IncrPendingTOTPAttempts(ctx, token); n != 1 {
		t.Fatalf("попытки %d", n)
	}
	if err := st.DeletePendingTOTP(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetPendingTOTP(ctx, token); err != ErrNotFound {
		t.Fatalf("ожидали ErrNotFound, %v", err)
	}
}

func TestMissingUsersAndCleanup(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	if err := st.SetRole(ctx, "missing", RoleAdmin); err != ErrNotFound {
		t.Errorf("SetRole: %v", err)
	}
	if err := st.SetDisabled(ctx, "missing", true); err != ErrNotFound {
		t.Errorf("SetDisabled: %v", err)
	}
	if err := st.DeleteSessionByID(ctx, "missing", "id"); err != nil {
		t.Errorf("DeleteSessionByID: %v", err)
	}
	if err := st.DeletePendingTOTP(ctx, ""); err != nil {
		t.Errorf("DeletePendingTOTP: %v", err)
	}
	if trusted, err := st.GetTrusted(ctx, "missing"); err != nil || len(trusted.Countries) != 0 {
		t.Errorf("GetTrusted: %+v %v", trusted, err)
	}
	if err := st.rdb.SAdd(ctx, usersIndex(), "ghost").Err(); err != nil {
		t.Fatal(err)
	}
	users, err := st.ListUsers(ctx)
	if err != nil || len(users) != 0 {
		t.Errorf("ListUsers: %+v %v", users, err)
	}
}
