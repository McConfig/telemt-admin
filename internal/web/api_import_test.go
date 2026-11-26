package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
)

func importRequest(t *testing.T, dryRun bool, body string, token, csrf string) *http.Request {
	t.Helper()
	path := "/api/users/import"
	if !dryRun {
		path += "?dryRun=0"
	}
	r := apiRequest(t, http.MethodPost, path, body, token, csrf)
	r.Header.Set("Content-Type", "text/csv; charset=utf-8")
	return r
}

func TestImportUsersPreviewAndApply(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	body := "name,max_unique_ips\n" +
		"alice,5\n" +
		"main,1\n" +
		"bad name,2\n" +
		"alice,3\n"

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, importRequest(t, true, body, tok, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("preview: код %d: %s", rr.Code, rr.Body.String())
	}
	var preview importResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.DryRun || preview.Total != 4 || preview.Create != 1 || preview.Skip != 1 || preview.Error != 2 {
		t.Fatalf("preview: %+v", preview)
	}
	if preview.Rows[0].Status != "create" || preview.Rows[1].Status != "skip" ||
		preview.Rows[2].Status != "error" || preview.Rows[3].Status != "error" {
		t.Fatalf("статусы: %+v", preview.Rows)
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, importRequest(t, false, body, tok, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("apply: код %d: %s", rr.Code, rr.Body.String())
	}
	var applied importResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &applied); err != nil {
		t.Fatal(err)
	}
	if applied.DryRun || applied.Create != 1 || applied.Skip != 1 || applied.Error != 2 {
		t.Fatalf("apply: %+v", applied)
	}
	events, _ := s.store.RecentAudit(context.Background(), 10)
	var importEvent bool
	for _, e := range events {
		if e.Action == "user.import" {
			importEvent = true
		}
	}
	if !importEvent {
		t.Fatalf("нет аудита user.import: %+v", events)
	}
}

func TestImportUsersDefaultDryRun(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	rr := httptest.NewRecorder()
	r := importRequest(t, true, "name\nalice\n", tok, csrf)
	r.URL.RawQuery = ""
	s.Handler().ServeHTTP(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}
	var out importResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if !out.DryRun || out.Create != 1 {
		t.Fatalf("ответ: %+v", out)
	}
}

func TestImportUsersErrors(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	for name, body := range map[string]string{
		"пусто":         "",
		"нет name":      "secret\nx\n",
		"битые кавычки": "name\n\"alice\n",
	} {
		t.Run(name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			s.Handler().ServeHTTP(rr, importRequest(t, true, body, tok, csrf))
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
			}
		})
	}

	big := "name\n" + strings.Repeat("user\n", 1001)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, importRequest(t, true, big, tok, csrf))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("лимит: код %d", rr.Code)
	}

	oversized := "name\n" + strings.Repeat("a", 1<<20)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, importRequest(t, true, oversized, tok, csrf))
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "1 МБ") {
		t.Fatalf("размер: код %d: %s", rr.Code, rr.Body.String())
	}
}

func TestImportUsersForbiddenForViewer(t *testing.T) {
	s, _, _ := coverageServer(t)
	viewer, viewerCSRF := seedSession(t, s, store.RoleViewer)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, importRequest(t, true, "name\nalice\n", viewer, viewerCSRF))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("код %d", rr.Code)
	}
}

func TestImportUsersTelemtFailure(t *testing.T) {
	s, tok, csrf := coverageServer(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/users" {
			_, _ = w.Write([]byte(`{"ok":true,"data":[]}`))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"unavailable","message":"нет связи"}}`))
	}))
	t.Cleanup(srv.Close)
	s.telemt = telemt.New(srv.URL, "", time.Second)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, importRequest(t, false, "name\nalice\n", tok, csrf))
	if rr.Code != http.StatusOK {
		t.Fatalf("код %d: %s", rr.Code, rr.Body.String())
	}
	var out importResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if out.Create != 0 || out.Error != 1 || out.Rows[0].Message == "" {
		t.Fatalf("ответ: %+v", out)
	}
}
