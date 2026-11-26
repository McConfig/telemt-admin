package store

import (
	"context"
	"testing"
	"time"
)

func TestQuotaResetStore(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)

	q, ok, err := st.GetQuotaReset(ctx, "alice")
	if err != nil || ok || q.Mode != "inherit" {
		t.Fatalf("по умолчанию: %+v %v %v", q, ok, err)
	}
	if err := st.SetQuotaReset(ctx, "alice", QuotaReset{Mode: "custom", Days: 7, Time: "05:00"}); err != nil {
		t.Fatal(err)
	}
	q, ok, err = st.GetQuotaReset(ctx, "alice")
	if err != nil || !ok || q.Mode != "custom" || q.Days != 7 || q.Time != "05:00" {
		t.Fatalf("после сохранения: %+v %v %v", q, ok, err)
	}
	if err := st.DeleteQuotaReset(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := st.GetQuotaReset(ctx, "alice"); ok {
		t.Fatal("переопределение должно быть удалено")
	}

	if _, ok, _ := st.QuotaLastRun(ctx, "alice"); ok {
		t.Fatal("время запуска не должно быть задано")
	}
	at := time.Date(2026, 11, 26, 4, 0, 0, 0, time.UTC)
	if err := st.SetQuotaLastRun(ctx, "alice", at); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.QuotaLastRun(ctx, "alice")
	if err != nil || !ok || !got.Equal(at) {
		t.Fatalf("время запуска: %s %v %v", got, ok, err)
	}

	locked, err := st.TryQuotaLock(ctx, "alice", 30*time.Second)
	if err != nil || !locked {
		t.Fatalf("первая блокировка: %v %v", locked, err)
	}
	locked, err = st.TryQuotaLock(ctx, "alice", 30*time.Second)
	if err != nil || locked {
		t.Fatalf("повторная блокировка: %v %v", locked, err)
	}
	st.UnlockQuota(ctx, "alice")
	locked, err = st.TryQuotaLock(ctx, "alice", 30*time.Second)
	if err != nil || !locked {
		t.Fatalf("после снятия: %v %v", locked, err)
	}
}
