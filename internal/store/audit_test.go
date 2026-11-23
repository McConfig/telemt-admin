package store

import (
	"context"
	"testing"
	"time"
)

func TestAuditStream(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	for i := 0; i < 3; i++ {
		if err := st.Audit(ctx, AuditEvent{
			At: time.Now().UTC(), Actor: "alice", Action: "user.create",
			Target: "bob", Result: "ok", IP: "192.0.2.10",
		}); err != nil {
			t.Fatalf("Audit: %v", err)
		}
	}
	got, err := st.RecentAudit(ctx, 2)
	if err != nil {
		t.Fatalf("RecentAudit: %v", err)
	}
	if len(got) != 2 || got[0].Action != "user.create" || got[0].Target != "bob" {
		t.Fatalf("получено %+v", got)
	}
}
