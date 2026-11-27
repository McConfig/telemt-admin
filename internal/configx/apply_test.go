package configx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/McConfig/telemt-admin/internal/telemt"
)

func TestApplySettingsMergeAndReplace(t *testing.T) {
	svc, _ := testService(t)
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec:     Spec{Settings: map[string]map[string]any{"risk": {"warn_ips": 4}}},
	}
	plan, err := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b})
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Apply(context.Background(), plan, ApplyOptions{
		Blocks: []Block{BlockSettings}, Modes: map[Block]string{BlockSettings: "merge"},
	})
	if err != nil || res[0].Status != "applied" || res[0].Applied != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	plan2, err := svc.Precheck(context.Background(), PrecheckOptions{
		Bundle: b, TargetSettingsVersion: svc.settings.Version(context.Background()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Apply(context.Background(), plan2, ApplyOptions{
		Blocks: []Block{BlockSettings}, Modes: map[Block]string{BlockSettings: "merge"},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestApplyRefusesStalePlan(t *testing.T) {
	svc, _ := testService(t)
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec:     Spec{Settings: map[string]map[string]any{"risk": {"warn_ips": 4}}},
	}
	plan, _ := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b})
	plan.TargetSettingsVersion--
	if _, err := svc.Apply(context.Background(), plan, ApplyOptions{Blocks: []Block{BlockSettings}}); !errors.Is(err, ErrStalePlan) {
		t.Fatalf("ожидалась stale_plan: %v", err)
	}
}

func TestApplyTemplatesAndDeleteMissing(t *testing.T) {
	svc, st := testService(t)
	seedExportData(t, st)
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec:     Spec{LimitTemplates: []LimitTemplate{{Name: "other"}}},
	}
	plan, err := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Apply(context.Background(), plan, ApplyOptions{
		Blocks: []Block{BlockTemplates}, DeleteMissing: true,
	}); err == nil {
		t.Fatal("ожидалась проверка confirmName")
	}
	plan, _ = svc.Precheck(context.Background(), PrecheckOptions{Bundle: b})
	res, err := svc.Apply(context.Background(), plan, ApplyOptions{
		Blocks: []Block{BlockTemplates}, DeleteMissing: true, ConfirmName: "demo",
	})
	if err != nil || res[0].Deleted != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if _, err := st.GetTemplate(context.Background(), "default"); !errors.Is(err, errors.New("not found")) {
		// GetTemplate returns ErrNotFound; проверяем только отсутствие default
		if _, err := st.GetTemplate(context.Background(), "default"); err == nil {
			t.Fatal("default не удалён")
		}
	}
}

func TestApplyUsersAndServices(t *testing.T) {
	svc, st := testService(t)
	seedExportData(t, st)
	hash, _ := hashPassword("new root password 123")
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsIncluded},
		Spec: Spec{
			Users: []User{
				{Name: "root", Role: "admin", PasswordHash: hash},
				{Name: "viewer", Role: "admin"},
			},
			Services: []ServiceAccount{{
				Name: "monitoring", Scope: "admin", Description: "обновлён",
				Tokens: []Token{{ID: "tok1", CreatedAt: "2026-11-20T10:00:00Z", SecretHash: "hash1"}},
			}},
		},
	}
	plan, err := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b})
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Apply(context.Background(), plan, ApplyOptions{
		Blocks:    []Block{BlockUsers, BlockServices},
		Conflicts: map[string]map[string]string{"users": {"viewer": ActionOverwrite}, "services": {"monitoring": ActionOverwrite}},
	})
	if err != nil {
		t.Fatalf("%+v", err)
	}
	if len(res) != 2 || res[0].Created != 1 || res[1].Updated != 1 {
		t.Fatalf("%+v", res)
	}
	root, err := st.GetUser(context.Background(), "root")
	if err != nil || root.Role != "admin" {
		t.Fatalf("%+v %v", root, err)
	}
	viewer, _ := st.GetUser(context.Background(), "viewer")
	if viewer.Role != "admin" {
		t.Fatalf("viewer: %+v", viewer)
	}
	tokens, err := st.ExportServiceTokens(context.Background(), "monitoring")
	found := false
	for _, token := range tokens {
		if token.ID == "tok1" && token.SecretHash == "hash1" {
			found = true
		}
	}
	if err != nil || !found {
		t.Fatalf("%+v %v", tokens, err)
	}
}

func TestExportProxyAndTemplates(t *testing.T) {
	svc, st := testService(t)
	seedExportData(t, st)
	b, err := svc.Export(context.Background(), ExportOptions{
		Blocks: []Block{BlockTemplates, BlockProxy}, Name: "demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Spec.LimitTemplates) != 1 || b.Spec.Proxy == nil || b.Spec.Proxy.Revision != "rev1" {
		t.Fatalf("%+v", b.Spec)
	}
}

func TestApplyProxy(t *testing.T) {
	svc, _ := testService(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/config":
			_, _ = w.Write([]byte(`{"ok":true,"revision":"rev1","data":{}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/config/validate":
			env := map[string]any{"ok": true, "data": map[string]any{}}
			_ = json.NewEncoder(w).Encode(env)
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/config":
			if r.Header.Get("If-Match") != "rev1" {
				w.WriteHeader(http.StatusConflict)
				return
			}
			_, _ = w.Write([]byte(`{"ok":true,"revision":"rev2","data":{"changed_paths":["censorship.tls_domain"],"reload":{"reload_id":3}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	svc.telemt = telemt.New(srv.URL, "", time.Second)
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec:     Spec{Proxy: &Proxy{Config: map[string]any{"censorship": map[string]any{"tls_domain": "new.example"}}}},
	}
	plan, err := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b})
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Apply(context.Background(), plan, ApplyOptions{Blocks: []Block{BlockProxy}})
	if err != nil || res[0].Status != "applied" {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestApplyClients(t *testing.T) {
	svc, st := testService(t)
	var mu sync.Mutex
	users := map[string]bool{"main": true, "old": true}
	patched := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/users":
			list := []map[string]any{}
			for name := range users {
				list = append(list, map[string]any{"username": name, "enabled": true})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": list})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/users":
			var in telemt.CreateInput
			_ = json.NewDecoder(r.Body).Decode(&in)
			users[in.Username] = true
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]any{
				"user": map[string]any{
					"username": in.Username, "enabled": true,
					"links": map[string]any{"tls": []string{"tg://proxy?server=demo.example.com&port=443&secret=0123456789abcdef0123456789abcdef"}},
				},
			}})
		case r.Method == http.MethodPatch:
			patched = true
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete:
			delete(users, r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:])
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	svc.telemt = telemt.New(srv.URL, "", time.Second)

	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec: Spec{Clients: []Client{
			{Username: "main", Enabled: true, Note: "обновлён"},
			{Username: "laptop", Enabled: true, Note: "новый", Trusted: &Trusted{Countries: []string{"GE"}}},
		}},
	}
	plan, err := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b})
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Apply(context.Background(), plan, ApplyOptions{
		Blocks:        []Block{BlockClients},
		Conflicts:     map[string]map[string]string{"clients": {"main": ActionOverwrite}},
		DeleteMissing: true, ConfirmName: "demo",
	})
	if err != nil {
		t.Fatalf("%+v %v", res, err)
	}
	if res[0].Created != 1 || res[0].Deleted != 1 || !patched {
		t.Fatalf("%+v", res[0])
	}
	if !users["laptop"] || users["old"] || !users["main"] {
		t.Fatalf("пользователи: %+v", users)
	}
	note, _ := st.Note(context.Background(), "laptop")
	if note != "новый" {
		t.Fatalf("заметка: %q", note)
	}
}

func TestApplyClientsPatchOmitsUnsetFields(t *testing.T) {
	svc, _ := testService(t)
	var mu sync.Mutex
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/users":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{{"username": "main", "enabled": true}}})
		case r.Method == http.MethodPatch:
			_ = json.NewDecoder(r.Body).Decode(&captured)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	svc.telemt = telemt.New(srv.URL, "", time.Second)
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec:     Spec{Clients: []Client{{Username: "main", Enabled: true, Note: "только заметка"}}},
	}
	plan, err := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Apply(context.Background(), plan, ApplyOptions{
		Blocks:    []Block{BlockClients},
		Conflicts: map[string]map[string]string{"clients": {"main": ActionOverwrite}},
	}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := captured["max_tcp_conns"]; ok {
		t.Fatalf("лимит попал в патч: %+v", captured)
	}
	if captured["enabled"] != true {
		t.Fatalf("enabled: %+v", captured)
	}
}

func TestApplyRefusesDeletingCurrentUser(t *testing.T) {
	svc, st := testService(t)
	seedExportData(t, st)
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec:     Spec{Users: []User{}},
	}
	plan, err := svc.Precheck(context.Background(), PrecheckOptions{Bundle: b})
	if err != nil {
		t.Fatal(err)
	}
	plan.Owner = "demo"
	_, err = svc.Apply(context.Background(), plan, ApplyOptions{
		Blocks: []Block{BlockUsers}, DeleteMissing: true, ConfirmName: "demo",
	})
	if err == nil {
		t.Fatal("ожидался запрет удаления текущего администратора")
	}
	if _, getErr := st.GetUser(context.Background(), "demo"); getErr != nil {
		t.Fatalf("demo удалён: %v", getErr)
	}
}
