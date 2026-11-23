package store

import (
	"context"
	"testing"
	"time"
)

func TestIPHistoryAndStats(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	t0 := time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)

	if err := st.RecordIPObservation(ctx, "alice", []string{"192.0.2.1", "192.0.2.2"}, 1000, t0); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordIPObservation(ctx, "alice", []string{"192.0.2.1", "192.0.2.3"}, 1500, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	stats, err := st.UserIPStats(ctx, "alice", t0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if stats.Distinct24h != 3 || stats.Distinct7d != 3 || stats.New24h != 3 {
		t.Fatalf("агрегаты: %+v", stats)
	}
	if stats.MaxToday != 2 {
		t.Fatalf("пик за день: %d", stats.MaxToday)
	}
	if stats.TrafficToday != 500 {
		t.Fatalf("трафик за день: %d", stats.TrafficToday)
	}

	history, err := st.IPHistory(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("история: %+v", history)
	}
	if !history[0].LastSeen.Equal(t0.Add(time.Minute)) && !history[0].LastSeen.Equal(t0) {
		t.Fatalf("порядок истории: %+v", history)
	}

	// Новые сутки: день обнуляется, дельта считается от последнего значения.
	next := t0.Add(24 * time.Hour)
	if err := st.RecordIPObservation(ctx, "alice", []string{"192.0.2.1"}, 1700, next); err != nil {
		t.Fatal(err)
	}
	stats, _ = st.UserIPStats(ctx, "alice", next)
	if stats.TrafficToday != 200 {
		t.Fatalf("трафик новых суток: %d", stats.TrafficToday)
	}
}
