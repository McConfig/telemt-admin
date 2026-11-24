package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthcheck(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ok.Close()
	t.Setenv("TGADMIN_HEALTH_URL", ok.URL)
	if err := healthcheck(); err != nil {
		t.Fatalf("healthcheck: %v", err)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer bad.Close()
	t.Setenv("TGADMIN_HEALTH_URL", bad.URL)
	if err := healthcheck(); err == nil {
		t.Fatal("ожидали ошибку для 503")
	}
}
