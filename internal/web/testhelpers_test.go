package web

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/alicebob/miniredis/v2"
)

func newTestStoreMR(t *testing.T) (*store.Store, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	st, err := store.Open(context.Background(), "redis://"+mr.Addr()+"/0", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st, mr
}

func testServerWithMR(t *testing.T) (*Server, *miniredis.Miniredis) {
	t.Helper()
	st, mr := newTestStoreMR(t)
	cfg := &config.Config{
		Listen: ":0", LinkMode: "tls", SessionTTL: time.Hour, CookieSecure: true,
		BaseURL:      mustURL("https://tgadmin.example.com"),
		AllowedCIDRs: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
	}
	return New(cfg, st, nil), mr
}
