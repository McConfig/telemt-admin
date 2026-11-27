package telemt

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetConfigReadsRevision(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/config" {
			t.Fatalf("путь %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ok":true,"revision":"abc","data":{"general":{"log_level":"normal"}}}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "", time.Second)
	got, err := c.GetConfig(context.Background())
	if err != nil || got.Revision != "abc" || got.Config["general"] == nil {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestPatchConfigSendsIfMatchAndReload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.Header.Get("If-Match") != "rev1" {
			t.Fatalf("%s %q", r.Method, r.Header.Get("If-Match"))
		}
		if r.URL.Query().Get("reload") != "true" {
			t.Fatalf("reload=%s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"ok":true,"revision":"rev2","data":{"changed_paths":["censorship.tls_domain"],"reload":{"reload_id":7,"state":"accepted"}}}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "", time.Second)
	got, err := c.PatchConfig(context.Background(), map[string]any{"censorship": map[string]any{"tls_domain": "x"}}, "rev1", true)
	if err != nil || got.Revision != "rev2" || got.ReloadID != 7 {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestPatchConfigMapsConflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"revision_conflict","message":"stale"}}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "", time.Second)
	_, err := c.PatchConfig(context.Background(), nil, "old", true)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != CodeConflict {
		t.Fatalf("%v", err)
	}
}

func TestGetConfigSchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"schema_version":1,"fields":[{"path":"general.log_level","api":{"config_read":true,"config_write":true}}]}}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "", time.Second)
	got, err := c.GetConfigSchema(context.Background())
	if err != nil || len(got.Fields) != 1 || !got.Fields[0].API.ConfigWrite {
		t.Fatalf("%+v %v", got, err)
	}
}
