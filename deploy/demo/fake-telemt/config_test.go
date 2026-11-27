//go:build demo

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfigPatchAndReload(t *testing.T) {
	s := newServer("", "demo.example.com")
	srv := httptest.NewServer(s.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/config")
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Revision string         `json:"revision"`
		Data     map[string]any `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&envelope)
	_ = resp.Body.Close()
	if envelope.Revision == "" || envelope.Data["general"] == nil {
		t.Fatalf("%+v", envelope)
	}

	body := []byte(`{"censorship":{"tls_domain":"new.example"}}`)
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/v1/config?reload=true", bytes.NewReader(body))
	req.Header.Set("If-Match", "stale")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("конфликт: %d", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodPatch, srv.URL+"/v1/config?reload=true", bytes.NewReader(body))
	req.Header.Set("If-Match", envelope.Revision)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var patched struct {
		Revision string `json:"revision"`
		Data     struct {
			ChangedPaths []string `json:"changed_paths"`
			Reload       struct {
				ReloadID int64 `json:"reload_id"`
			} `json:"reload"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&patched)
	_ = resp.Body.Close()
	if patched.Revision == envelope.Revision || patched.Data.Reload.ReloadID == 0 {
		t.Fatalf("%+v", patched)
	}

	resp, err = http.Get(srv.URL + "/v1/system/reload/1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("статус перезагрузки: %d", resp.StatusCode)
	}
}
