package store

import (
	"context"
	"testing"
	"time"
)

func TestLoginThrottle(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)

	max, _ := st.LoginPolicy()
	for i := 0; i < max; i++ {
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
		t.Fatalf("после %d неудач: %v, %v", max, ok, err)
	}
	if err := st.ResetLoginFailures(ctx, "192.0.2.10", "alice"); err != nil {
		t.Fatalf("ResetLoginFailures: %v", err)
	}
	ok, _ = st.LoginAllowed(ctx, "192.0.2.10", "alice")
	if !ok {
		t.Fatal("после сброса вход должен быть разрешён")
	}
}

func TestLoginPolicyCanBeChanged(t *testing.T) {
	ctx := context.Background()
	st, mr := newTestStore(t)
	st.SetLoginPolicy(2, time.Minute)
	for i := 0; i < 2; i++ {
		if err := st.RecordLoginFailure(ctx, "192.0.2.10", "alice"); err != nil {
			t.Fatal(err)
		}
	}
	if ok, _ := st.LoginAllowed(ctx, "192.0.2.10", "alice"); ok {
		t.Fatal("после двух неудач вход должен быть заблокирован")
	}
	mr.FastForward(time.Minute + time.Second)
	if ok, _ := st.LoginAllowed(ctx, "192.0.2.10", "alice"); !ok {
		t.Fatal("после окна вход должен быть разрешён")
	}
}
