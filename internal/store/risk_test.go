package store

import (
	"context"
	"testing"
)

func TestRiskDefaultAndOverrides(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)

	if _, ok, err := st.GetRisk(ctx, "alice"); err != nil || ok {
		t.Fatalf("пустое хранилище: ok=%v err=%v", ok, err)
	}

	zero, high, distinct := 0, 8, 20
	no := false
	want := Risk{WarnIPs: &zero, HighIPs: &high, Distinct7d: &distinct, NewCountries: &no}
	if err := st.SetRisk(ctx, "alice", want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.GetRisk(ctx, "alice")
	if err != nil || !ok {
		t.Fatalf("GetRisk: ok=%v err=%v", ok, err)
	}
	if got.WarnIPs == nil || *got.WarnIPs != 0 {
		t.Fatalf("warn: %+v", got.WarnIPs)
	}
	if got.HighIPs == nil || *got.HighIPs != 8 {
		t.Fatalf("high: %+v", got.HighIPs)
	}
	if got.Distinct7d == nil || *got.Distinct7d != 20 {
		t.Fatalf("distinct: %+v", got.Distinct7d)
	}
	if got.NewCountries == nil || *got.NewCountries {
		t.Fatalf("newCountries: %+v", got.NewCountries)
	}

	// Частичное обновление не должно оставлять старые поля.
	onlyHigh := 4
	if err := st.SetRisk(ctx, "alice", Risk{HighIPs: &onlyHigh}); err != nil {
		t.Fatal(err)
	}
	got, _, _ = st.GetRisk(ctx, "alice")
	if got.WarnIPs != nil || got.Distinct7d != nil || got.NewCountries != nil {
		t.Fatalf("старые поля остались: %+v", got)
	}
	if got.HighIPs == nil || *got.HighIPs != 4 {
		t.Fatalf("high после замены: %+v", got.HighIPs)
	}

	if err := st.DeleteRisk(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := st.GetRisk(ctx, "alice"); err != nil || ok {
		t.Fatalf("после удаления: ok=%v err=%v", ok, err)
	}
}

func TestSetEmptyRiskDeletes(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)

	high := 6
	if err := st.SetRisk(ctx, "bob", Risk{HighIPs: &high}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetRisk(ctx, "bob", Risk{}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := st.GetRisk(ctx, "bob"); err != nil || ok {
		t.Fatalf("пустая структура должна удалять: ok=%v err=%v", ok, err)
	}
}
