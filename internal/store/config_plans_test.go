package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestConfigPlanLifecycle(t *testing.T) {
	st, _ := newTestStore(t)
	ctx := context.Background()
	p := ConfigPlan{ID: "abc", Owner: "demo", Session: "sess", Payload: []byte(`{"x":1}`), CreatedAt: time.Now()}
	if err := st.PutConfigPlan(ctx, p, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetConfigPlan(ctx, "abc")
	if err != nil || got.Owner != "demo" || string(got.Payload) != `{"x":1}` {
		t.Fatalf("%+v %v", got, err)
	}
	taken, err := st.TakeConfigPlan(ctx, "abc")
	if err != nil || taken.ID != "abc" {
		t.Fatalf("%+v %v", taken, err)
	}
	if _, err := st.TakeConfigPlan(ctx, "abc"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("повторный Take: %v", err)
	}
}
