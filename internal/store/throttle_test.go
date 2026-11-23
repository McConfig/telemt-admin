package store

import (
	"context"
	"testing"
)

func TestLoginThrottle(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)

	for i := 0; i < maxLoginFailures; i++ {
		ok, err := st.LoginAllowed(ctx, "192.0.2.10", "alice")
		if err != nil || !ok {
			t.Fatalf("попытка %d: %v, %v", i, ok, err)
		}
		if err := st.RecordLoginFailure(ctx, "192.0.2.10", "alice"); err != nil {
			t.Fatalf("RecordLoginFailure: %v", err)
		}
	}
	ok, err := st.LoginAllowed(ctx, "192.0.2.10", "alice")
	if err != nil || ok {
		t.Fatalf("после %d неудач: %v, %v", maxLoginFailures, ok, err)
	}
	if err := st.ResetLoginFailures(ctx, "192.0.2.10", "alice"); err != nil {
		t.Fatalf("ResetLoginFailures: %v", err)
	}
	ok, _ = st.LoginAllowed(ctx, "192.0.2.10", "alice")
	if !ok {
		t.Fatal("после сброса вход должен быть разрешён")
	}
}
