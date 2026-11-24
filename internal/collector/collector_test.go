package collector

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
	"github.com/alicebob/miniredis/v2"
)

func TestCollectAndMetrics(t *testing.T) {
	mr := miniredis.RunT(t)
	st, err := store.Open(context.Background(), "redis://"+mr.Addr()+"/0", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"data":[
		 {"username":"alice","enabled":true,"total_octets":5000,
		  "active_unique_ips_list":["192.0.2.1","192.0.2.2","192.0.2.3","192.0.2.4"]}
		]}`))
	}))
	defer srv.Close()

	c := New(st, telemt.New(srv.URL, "", 5*time.Second), nil, time.Hour, 3, 5, 15)
	c.collect(context.Background())

	snap, ok := c.Stats("alice")
	if !ok || snap.Current != 4 || snap.Risk != 1 {
		t.Fatalf("срез: %+v, %v", snap, ok)
	}
	if snap.TrafficTotal != 5000 {
		t.Fatalf("трафик: %d", snap.TrafficTotal)
	}

	rr := httptest.NewRecorder()
	c.ServeHTTP(rr, httptest.NewRequest("GET", "/metrics", nil))
	body, _ := io.ReadAll(rr.Result().Body)
	for _, want := range []string{
		`tgadmin_user_ips_current{user="alice"} 4`,
		`tgadmin_user_risk_level{user="alice"} 1`,
		`tgadmin_user_traffic_bytes_total{user="alice"} 5000`,
		"tgadmin_collector_up 1",
		"tgadmin_geoip_loaded 0",
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("нет строки %q в метриках:\n%s", want, body)
		}
	}
}

func TestCollectorRunAndStatus(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mr := miniredis.RunT(t)
	st, err := store.Open(context.Background(), "redis://"+mr.Addr()+"/0", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"username":"alice","enabled":true,"total_octets":10}]}`))
	}))
	defer srv.Close()

	c := New(st, telemt.New(srv.URL, "", time.Second), nil, 10*time.Millisecond, 3, 5, 15)
	go c.Run(ctx)
	deadline := time.Now().Add(2 * time.Second)
	for !c.Status().Up && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	time.Sleep(20 * time.Millisecond)

	status := c.Status()
	if !status.Up || status.LastSuccess.IsZero() || status.LastError != "" {
		t.Fatalf("статус: %+v", status)
	}
	if snap := c.Snapshot(); len(snap) != 1 {
		t.Fatalf("срез: %+v", snap)
	}
	if _, ok := c.Stats("missing"); ok {
		t.Fatal("найден несуществующий клиент")
	}
}

func TestCollectorFailureStatus(t *testing.T) {
	mr := miniredis.RunT(t)
	st, err := store.Open(context.Background(), "redis://"+mr.Addr()+"/0", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := New(st, telemt.New(srv.URL, "", time.Second), nil, time.Hour, 3, 5, 15)
	c.collect(context.Background())
	status := c.Status()
	if status.Up || status.LastError == "" {
		t.Fatalf("статус ошибки: %+v", status)
	}
}
