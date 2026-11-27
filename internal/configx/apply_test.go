package configx

import (
	"context"
	"errors"
	"testing"
)

func TestApplySettingsMergeAndReplace(t *testing.T) {
	svc, _ := testService(t)
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec:     Spec{Settings: map[string]map[string]any{"risk": {"warn_ips": 4}}},
	}
	plan, err := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b})
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Apply(context.Background(), plan, ApplyOptions{
		Blocks: []Block{BlockSettings}, Modes: map[Block]string{BlockSettings: "merge"},
	})
	if err != nil || res[0].Status != "applied" || res[0].Applied != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	plan2, err := svc.Precheck(context.Background(), PrecheckOptions{
		Bundle: b, TargetSettingsVersion: svc.settings.Version(context.Background()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Apply(context.Background(), plan2, ApplyOptions{
		Blocks: []Block{BlockSettings}, Modes: map[Block]string{BlockSettings: "merge"},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestApplyRefusesStalePlan(t *testing.T) {
	svc, _ := testService(t)
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec:     Spec{Settings: map[string]map[string]any{"risk": {"warn_ips": 4}}},
	}
	plan, _ := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b})
	plan.TargetSettingsVersion--
	if _, err := svc.Apply(context.Background(), plan, ApplyOptions{Blocks: []Block{BlockSettings}}); !errors.Is(err, ErrStalePlan) {
		t.Fatalf("ожидалась stale_plan: %v", err)
	}
}

func TestApplyTemplatesAndDeleteMissing(t *testing.T) {
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
	if _, err := svc.Apply(context.Background(), plan, ApplyOptions{
		Blocks: []Block{BlockTemplates}, DeleteMissing: true,
	}); err == nil {
		t.Fatal("ожидалась проверка confirmName")
	}
	plan, _ = svc.Precheck(context.Background(), PrecheckOptions{Bundle: b})
	res, err := svc.Apply(context.Background(), plan, ApplyOptions{
		Blocks: []Block{BlockTemplates}, DeleteMissing: true, ConfirmName: "demo",
	})
	if err != nil || res[0].Deleted != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if _, err := st.GetTemplate(context.Background(), "default"); !errors.Is(err, errors.New("not found")) {
		// GetTemplate returns ErrNotFound; проверяем только отсутствие default
		if _, err := st.GetTemplate(context.Background(), "default"); err == nil {
			t.Fatal("default не удалён")
		}
	}
}
