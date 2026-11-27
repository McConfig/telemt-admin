package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/configx"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/totp"
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

func TestConfigBlocksAndPlanSuccess(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	rr := call(t, s, "GET", "/api/config/blocks", "", tok, "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "limitTemplates") {
		t.Fatalf("blocks: %d %s", rr.Code, rr.Body.String())
	}
	yaml := "apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata:\n  name: demo\n  secrets: none\nspec:\n  settings:\n    risk:\n      warn_ips: 4\n"
	body, _ := json.Marshal(map[string]any{"yaml": yaml, "fileName": "x.yaml"})
	rr = call(t, s, "POST", "/api/config/precheck", string(body), tok, csrf)
	var plan struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &plan)
	rr = call(t, s, "GET", "/api/config/plan/"+plan.ID, "", tok, "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "risk.warn_ips") {
		t.Fatalf("plan: %d %s", rr.Code, rr.Body.String())
	}
	rr = call(t, s, "GET", "/api/config/plan/missing", "", tok, "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("чужой план: %d", rr.Code)
	}
}

func TestConfigExportSecrets(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	rr := call(t, s, "POST", "/api/config/export",
		`{"blocks":["settings"],"secrets":true,"password":"wrong"}`, tok, csrf)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("неверный пароль: %d %s", rr.Code, rr.Body.String())
	}
	rr = call(t, s, "POST", "/api/config/export",
		`{"blocks":["settings"],"secrets":true,"password":"long password 123"}`, tok, csrf)
	if rr.Code != http.StatusOK {
		t.Fatalf("экспорт секретов: %d %s", rr.Code, rr.Body.String())
	}
	rr = call(t, s, "POST", "/api/config/export", `{"blocks":["nope"]}`, tok, csrf)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("неизвестный блок: %d", rr.Code)
	}
}

func TestConfigApplyStalePlan(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	yaml := "apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata:\n  name: demo\n  secrets: none\nspec:\n  settings:\n    risk:\n      warn_ips: 4\n"
	body, _ := json.Marshal(map[string]any{"yaml": yaml})
	rr := call(t, s, "POST", "/api/config/precheck", string(body), tok, csrf)
	var plan struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &plan)
	if _, _, err := s.settings.Save(context.Background(), s.settings.Version(context.Background()),
		map[string]json.RawMessage{"risk.high_ips": json.RawMessage(`6`)}, nil); err != nil {
		t.Fatal(err)
	}
	apply, _ := json.Marshal(map[string]any{"planId": plan.ID, "blocks": []string{"settings"}})
	rr = call(t, s, "POST", "/api/config/apply", string(apply), tok, csrf)
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "stale_plan") {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestConfigExportRateLimited(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	var last int
	for i := 0; i < 12; i++ {
		last = call(t, s, "POST", "/api/config/export", `{"blocks":["settings"]}`, tok, csrf).Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("ожидался 429, получен %d", last)
	}
}

func TestConfigPrecheckSecretsWithPassword(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	yaml := "apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata:\n  name: demo\n  secrets: included\nspec:\n  users:\n    - name: user-admin\n      role: admin\n"
	body, _ := json.Marshal(map[string]any{"yaml": yaml, "password": "long password 123"})
	rr := call(t, s, "POST", "/api/config/precheck", string(body), tok, csrf)
	if rr.Code != http.StatusOK {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	body, _ = json.Marshal(map[string]any{"yaml": yaml, "password": "wrong"})
	rr = call(t, s, "POST", "/api/config/precheck", string(body), tok, csrf)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("неверный пароль: %d %s", rr.Code, rr.Body.String())
	}
}

func TestConfigApplyWithoutBlocks(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	yaml := "apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata:\n  name: demo\n  secrets: none\nspec:\n  settings: {}\n"
	body, _ := json.Marshal(map[string]any{"yaml": yaml})
	rr := call(t, s, "POST", "/api/config/precheck", string(body), tok, csrf)
	var plan struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &plan)
	apply, _ := json.Marshal(map[string]any{"planId": plan.ID, "blocks": []string{}})
	rr = call(t, s, "POST", "/api/config/apply", string(apply), tok, csrf)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestConfigExportSecretsWithTOTP(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	secret, _ := totp.GenerateSecret()
	enc, _ := totp.Encrypt(s.cfg.TOTPKey, secret)
	if err := s.store.EnableTOTP(context.Background(), "user-admin", enc, 0, nil); err != nil {
		t.Fatal(err)
	}
	code, _ := totp.Code(secret, time.Now())
	rr := call(t, s, "POST", "/api/config/export",
		`{"blocks":["settings"],"secrets":true,"password":"long password 123","code":"`+code+`"}`, tok, csrf)
	if rr.Code != http.StatusOK {
		t.Fatalf("код TOTP: %d %s", rr.Code, rr.Body.String())
	}
	rr = call(t, s, "POST", "/api/config/export",
		`{"blocks":["settings"],"secrets":true,"password":"long password 123","code":"000000"}`, tok, csrf)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("неверный код: %d %s", rr.Code, rr.Body.String())
	}
}

func TestConfigPlanOtherOwner(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	yaml := "apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata:\n  name: demo\n  secrets: none\nspec:\n  settings: {}\n"
	body, _ := json.Marshal(map[string]any{"yaml": yaml})
	rr := call(t, s, "POST", "/api/config/precheck", string(body), tok, csrf)
	var plan struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &plan)
	other, otherCSRF := seedSession(t, s, store.RoleAdmin)
	rr = call(t, s, "GET", "/api/config/plan/"+plan.ID, "", other, otherCSRF)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestConfigPrecheckRateLimited(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	body, _ := json.Marshal(map[string]any{"yaml": "apiVersion: nope\n"})
	var last int
	for i := 0; i < 7; i++ {
		last = call(t, s, "POST", "/api/config/precheck", string(body), tok, csrf).Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("ожидался 429, получен %d", last)
	}
}

func TestConfigPrecheckInvalidSetting(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	yaml := "apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata:\n  name: demo\n  secrets: none\nspec:\n  settings:\n    risk:\n      warn_ips: -1\n"
	body, _ := json.Marshal(map[string]any{"yaml": yaml})
	rr := call(t, s, "POST", "/api/config/precheck", string(body), tok, csrf)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "мин") {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestConfigApplyMissingPlan(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	rr := call(t, s, "POST", "/api/config/apply", `{"planId":"nope","blocks":["settings"]}`, tok, csrf)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestConfigExportProxyUnavailable(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	s.configx = configx.New(s.store, s.settings, nil, time.Now)
	rr := call(t, s, "POST", "/api/config/export", `{"blocks":["proxy"]}`, tok, csrf)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "proxy_unavailable") {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestConfigApplyForeignPlan(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	yaml := "apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata:\n  name: demo\n  secrets: none\nspec:\n  settings: {}\n"
	body, _ := json.Marshal(map[string]any{"yaml": yaml})
	rr := call(t, s, "POST", "/api/config/precheck", string(body), tok, csrf)
	var plan struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &plan)
	other, otherCSRF := seedSession(t, s, store.RoleAdmin)
	apply, _ := json.Marshal(map[string]any{"planId": plan.ID, "blocks": []string{"settings"}})
	rr = call(t, s, "POST", "/api/config/apply", string(apply), other, otherCSRF)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}
