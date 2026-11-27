package configx

import (
	"context"
	"testing"

	"github.com/McConfig/telemt-admin/internal/store"
)

func TestPrecheckBuildsDiffAndConflicts(t *testing.T) {
	svc, st := testService(t)
	seedExportData(t, st)
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec: Spec{
			Settings: map[string]map[string]any{"risk": {"warn_ips": 4}},
			Users:    []User{{Name: "viewer", Role: "admin"}},
		},
	}
	plan, err := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b, FileName: "x.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Diff["settings"]) != 1 || plan.Diff["settings"][0].Path != "risk.warn_ips" {
		t.Fatalf("diff=%+v", plan.Diff)
	}
	if len(plan.Conflicts["users"]) != 1 || plan.Conflicts["users"][0].Action != ActionKeep {
		t.Fatalf("конфликты=%+v", plan.Conflicts)
	}
	if plan.Conflicts["users"][0].Name != "viewer" {
		t.Fatalf("имя конфликта: %+v", plan.Conflicts["users"])
	}
}

func TestPrecheckRejectsLastAdminDemotion(t *testing.T) {
	svc, st := testService(t)
	seedExportData(t, st)
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec:     Spec{Users: []User{{Name: "demo", Role: "viewer", Disabled: true}}},
	}
	if _, err := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b}); err == nil {
		t.Fatal("ожидалась защита последнего администратора")
	}
}

func TestPrecheckRejectsUnknownProxySection(t *testing.T) {
	svc, _ := testService(t)
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec:     Spec{Proxy: &Proxy{Config: map[string]any{"access": map[string]any{}}}},
	}
	if _, err := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b}); err == nil {
		t.Fatal("ожидалась ошибка секции access")
	}
}

func TestPrecheckDeletions(t *testing.T) {
	svc, st := testService(t)
	seedExportData(t, st)
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec:     Spec{LimitTemplates: []LimitTemplate{{Name: "other"}}},
	}
	plan, err := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Deletions["limitTemplates"]) != 1 || plan.Deletions["limitTemplates"][0] != "default" {
		t.Fatalf("%+v", plan.Deletions)
	}
	_ = store.RoleAdmin
}

func TestPrecheckRejectsSecretsWithoutFlag(t *testing.T) {
	svc, _ := testService(t)
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec:     Spec{Clients: []Client{{Username: "main", Secret: "0123456789abcdef0123456789abcdef"}}},
	}
	if _, err := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b}); err == nil {
		t.Fatal("ожидалась ошибка секрета без флага")
	}
}
