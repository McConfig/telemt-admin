package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigExportRequiresSession(t *testing.T) {
	s, _, _ := coverageServer(t)
	rr := call(t, s, "POST", "/api/config/export", `{"blocks":["settings"]}`, "", "")
	if rr.Code != http.StatusUnauthorized && rr.Code != http.StatusForbidden {
		t.Fatalf("без сессии: %d", rr.Code)
	}
}

func TestConfigExportSettings(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	rr := call(t, s, "POST", "/api/config/export", `{"blocks":["settings"]}`, tok, csrf)
	if rr.Code != http.StatusOK {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Content-Disposition") == "" || rr.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("заголовки: %v", rr.Header())
	}
	if !strings.Contains(rr.Body.String(), "apiVersion: tgadmin.telemt/v1") {
		t.Fatalf("тело: %s", rr.Body.String())
	}
}

func TestConfigPrecheckAndApply(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	yaml := "apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata:\n  name: demo\n  secrets: none\nspec:\n  settings:\n    risk:\n      warn_ips: 4\n"
	body, _ := json.Marshal(map[string]any{"yaml": yaml, "fileName": "x.yaml"})
	rr := call(t, s, "POST", "/api/config/precheck", string(body), tok, csrf)
	if rr.Code != http.StatusOK {
		t.Fatalf("precheck: %d %s", rr.Code, rr.Body.String())
	}
	var plan struct {
		ID     string `json:"id"`
		Blocks []struct {
			ID string `json:"id"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil || plan.ID == "" {
		t.Fatalf("%+v %v", plan, err)
	}
	apply, _ := json.Marshal(map[string]any{
		"planId": plan.ID, "blocks": []string{"settings"},
		"modes": map[string]string{"settings": "merge"},
	})
	rr = call(t, s, "POST", "/api/config/apply", string(apply), tok, csrf)
	if rr.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"status":"applied"`) {
		t.Fatalf("результат: %s", rr.Body.String())
	}
	rr = call(t, s, "POST", "/api/config/apply", string(apply), tok, csrf)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("повторный apply: %d", rr.Code)
	}
}

func TestConfigPrecheckRejectsSecretsWithoutPassword(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	yaml := "apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata:\n  name: demo\n  secrets: included\nspec:\n  users:\n    - name: demo\n      role: admin\n"
	body, _ := json.Marshal(map[string]any{"yaml": yaml})
	rr := call(t, s, "POST", "/api/config/precheck", string(body), tok, csrf)
	if rr.Code != http.StatusForbidden || !bytes.Contains(rr.Body.Bytes(), []byte("secrets_confirmation_required")) {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestConfigPrecheckRejectsBadYAML(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	body, _ := json.Marshal(map[string]any{"yaml": "apiVersion: nope\n"})
	rr := call(t, s, "POST", "/api/config/precheck", string(body), tok, csrf)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestConfigPlanForeignSession(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	yaml := "apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata:\n  name: demo\n  secrets: none\nspec:\n  settings:\n    risk:\n      warn_ips: 4\n"
	body, _ := json.Marshal(map[string]any{"yaml": yaml})
	rr := call(t, s, "POST", "/api/config/precheck", string(body), tok, csrf)
	var plan struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &plan)
	other, otherCSRF := seedSession(t, s, "viewer")
	rr = call(t, s, "GET", "/api/config/plan/"+plan.ID, "", other, otherCSRF)
	if rr.Code != http.StatusUnauthorized && rr.Code != http.StatusForbidden {
		t.Fatalf("viewer: %d", rr.Code)
	}
	_ = httptest.NewRecorder()
}
