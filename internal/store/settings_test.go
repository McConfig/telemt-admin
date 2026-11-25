package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGlobalSettingsSaveAndConflict(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)

	if _, version, err := st.GlobalSettings(ctx); err != nil || version != 0 {
		t.Fatalf("начальная версия: %d %v", version, err)
	}
	version, err := st.SaveGlobalSettings(ctx, 0, map[string]string{"risk.warn_ips": "4"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("версия %d", version)
	}
	overrides, gotVersion, err := st.GlobalSettings(ctx)
	if err != nil || gotVersion != 1 || overrides["risk.warn_ips"] != "4" {
		t.Fatalf("после сохранения: %+v %d %v", overrides, gotVersion, err)
	}

	if _, err := st.SaveGlobalSettings(ctx, 0, map[string]string{"risk.warn_ips": "5"}, nil); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("ожидался конфликт: %v", err)
	}

	version, err = st.SaveGlobalSettings(ctx, 1, nil, []string{"risk.warn_ips"})
	if err != nil {
		t.Fatal(err)
	}
	overrides, _, _ = st.GlobalSettings(ctx)
	if version != 2 || len(overrides) != 0 {
		t.Fatalf("после сброса: версия %d, %+v", version, overrides)
	}
}

func TestTemplatesCRUD(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)

	ips := uint64(5)
	tpl := LimitTemplate{Name: "router", MaxIps: &ips}
	if err := st.SaveTemplate(ctx, tpl); err != nil {
		t.Fatal(err)
	}
	list, err := st.ListTemplates(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "router" {
		t.Fatalf("список: %+v %v", list, err)
	}
	got, err := st.GetTemplate(ctx, "router")
	if err != nil || got.MaxIps == nil || *got.MaxIps != 5 {
		t.Fatalf("GetTemplate: %+v %v", got, err)
	}
	if err := st.SetDefaultTemplate(ctx, "router"); err != nil {
		t.Fatal(err)
	}
	if def, _ := st.DefaultTemplate(ctx); def != "router" {
		t.Fatalf("default: %s", def)
	}

	other := "other"
	if err := st.SaveTemplate(ctx, LimitTemplate{Name: other}); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteTemplate(ctx, "router"); err != nil {
		t.Fatal(err)
	}
	if def, _ := st.DefaultTemplate(ctx); def != "" {
		t.Fatalf("default после удаления: %s", def)
	}
	if _, err := st.GetTemplate(ctx, "router"); !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("удалённый шаблон: %v", err)
	}
}

func TestHomeCountryAffectsForeign(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	day := now.Format("2006-01-02")

	st.SetHomeCountry("DE")
	geo := map[string]IPGeo{
		"192.0.2.1": {Country: "DE", CountryName: "Германия"},
		"192.0.2.2": {Country: "NL", CountryName: "Нидерланды"},
	}
	if err := st.RecordIPObservation(ctx, "alice", []string{"192.0.2.1", "192.0.2.2"}, geo, 100, now); err != nil {
		t.Fatal(err)
	}
	foreign, err := st.rdb.SCard(ctx, foreignKey("alice", day)).Result()
	if err != nil {
		t.Fatal(err)
	}
	if foreign != 1 {
		t.Fatalf("иностранных %d, ожидался 1", foreign)
	}
	if st.HomeCountry() != "DE" {
		t.Fatalf("домашняя страна: %s", st.HomeCountry())
	}
}
