package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/alicebob/miniredis/v2"
)

func TestNewHandlerServesAPI(t *testing.T) {
	mr := miniredis.RunT(t)
	cfg := &config.Config{
		Listen: ":0", LinkMode: "tls", SessionTTL: time.Hour,
		BaseURL:      mustParseURL("https://tgadmin.example.com"),
		AllowedCIDRs: mustCIDRs("127.0.0.1/32"),
		ValkeyURL:    "redis://" + mr.Addr() + "/0",
		TelemtURL:    "http://127.0.0.1:1",
	}
	h, closeFn, err := newHandler(context.Background(), cfg)
	if err != nil {
		t.Fatalf("newHandler: %v", err)
	}
	defer func() { _ = closeFn() }()

	req := httptest.NewRequest("GET", "/api/me", nil)
	req.RemoteAddr = "127.0.0.1:5555"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}
	if !contains(rr.Body.String(), "unauthorized") {
		t.Fatalf("нет JSON-ошибки: %s", rr.Body.String())
	}
}

func TestHandlerHealthz(t *testing.T) {
	mr := miniredis.RunT(t)
	cfg := &config.Config{
		Listen: ":0", LinkMode: "tls", SessionTTL: time.Hour,
		BaseURL:      mustParseURL("https://tgadmin.example.com"),
		AllowedCIDRs: mustCIDRs("127.0.0.1/32"),
		ValkeyURL:    "redis://" + mr.Addr() + "/0",
		TelemtURL:    "http://127.0.0.1:1",
	}
	h, closeFn, err := newHandler(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closeFn() }()
	req := httptest.NewRequest("GET", "/healthz", nil)
	req.RemoteAddr = "127.0.0.1:5555"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Body.String() != "ok\n" {
		t.Fatalf("healthz: %d %q", rr.Code, rr.Body.String())
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

var _ = context.Background
