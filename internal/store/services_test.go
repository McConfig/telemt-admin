package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestServiceCRUD(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)

	svc := ServiceAccount{
		Name: "monitoring", Description: "Prometheus", Scope: ScopeRead, CreatedBy: "admin",
	}
	if err := st.CreateService(ctx, svc); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateService(ctx, svc); !errors.Is(err, ErrExists) {
		t.Fatalf("повтор: %v", err)
	}
	got, err := st.GetService(ctx, "monitoring")
	if err != nil || got.Scope != ScopeRead || got.Disabled {
		t.Fatalf("get: %+v %v", got, err)
	}
	scope := ScopeWrite
	disabled := true
	got, err = st.UpdateService(ctx, "monitoring", ServiceUpdate{Scope: &scope, Disabled: &disabled})
	if err != nil || got.Scope != ScopeWrite || !got.Disabled {
		t.Fatalf("update: %+v %v", got, err)
	}
	list, err := st.ListServices(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "monitoring" {
		t.Fatalf("list: %+v %v", list, err)
	}
	if err := st.DeleteService(ctx, "monitoring"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetService(ctx, "monitoring"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("после удаления: %v", err)
	}
}

func TestServiceLimit(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	for i := 0; i < MaxServiceAccounts; i++ {
		name := fmt.Sprintf("svc%02d", i)
		if err := st.CreateService(ctx, ServiceAccount{Name: name, Scope: ScopeRead}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.CreateService(ctx, ServiceAccount{Name: "svc-extra", Scope: ScopeRead}); !errors.Is(err, ErrLimit) {
		t.Fatalf("лимит: %v", err)
	}
}

func TestUpdateMissingService(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	if _, err := st.UpdateService(ctx, "missing", ServiceUpdate{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update missing: %v", err)
	}
	if err := st.DeleteService(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete missing: %v", err)
	}
}

func TestCreateServiceConcurrentLimit(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	var wg sync.WaitGroup
	for i := 0; i < MaxServiceAccounts+10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = st.CreateService(ctx, ServiceAccount{
				Name: fmt.Sprintf("svc%02d", i), Scope: ScopeRead,
			})
		}(i)
	}
	wg.Wait()
	list, err := st.ListServices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) > MaxServiceAccounts {
		t.Fatalf("сервисов %d, лимит %d", len(list), MaxServiceAccounts)
	}
}
