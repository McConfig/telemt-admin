//go:build demo

// Command fake-telemt — демонстрационный Control API telemt.
//
// Сервис нужен только для локального демо и скриншотов: он не проксирует
// трафик, не хранит реальные секреты и отвечает теми же JSON-структурами,
// что настоящий telemt. Данные детерминированные, адреса — из RFC 5737.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultListen = ":9091"
	defaultDomain = "demo.example.com"
)

// Links — ссылки пользователя.
type Links struct {
	Classic []string `json:"classic"`
	Secure  []string `json:"secure"`
	TLS     []string `json:"tls"`
}

// User — пользователь прокси в формате Control API.
type User struct {
	Username           string   `json:"username"`
	Enabled            bool     `json:"enabled"`
	InRuntime          bool     `json:"in_runtime"`
	MaxTCPConns        *uint64  `json:"max_tcp_conns"`
	MaxUniqueIPs       *uint64  `json:"max_unique_ips"`
	ExpirationRFC3339  *string  `json:"expiration_rfc3339"`
	DataQuotaBytes     *uint64  `json:"data_quota_bytes"`
	QuotaPeriod        *string  `json:"quota_period"`
	RateLimitUpBps     *uint64  `json:"rate_limit_up_bps"`
	RateLimitDownBps   *uint64  `json:"rate_limit_down_bps"`
	CurrentConnections uint64   `json:"current_connections"`
	ActiveUniqueIPs    uint64   `json:"active_unique_ips"`
	ActiveUniqueIPList []string `json:"active_unique_ips_list"`
	RecentUniqueIPList []string `json:"recent_unique_ips_list"`
	TotalOctets        uint64   `json:"total_octets"`
	Links              *Links   `json:"links"`
}

type server struct {
	mu        sync.Mutex
	token     string
	domain    string
	users     map[string]*User
	order     []string
	rots      int
	config    map[string]any
	revision  string
	reloads   map[int64]string
	reloadSeq int64
}

func main() {
	listen := env("TELEMT_LISTEN", defaultListen)
	s := newServer(env("TELEMT_API_TOKEN", ""), env("DEMO_DOMAIN", defaultDomain))

	log.Printf("fake telemt слушает %s, домен ссылок %s", listen, s.domain)
	srv := &http.Server{
		Addr:              listen,
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func newServer(token, domain string) *server {
	s := &server{
		token: token, domain: domain, users: map[string]*User{},
		reloads: map[int64]string{},
		config: map[string]any{
			"general": map[string]any{
				"links": map[string]any{"public_host": domain, "public_port": 443.0},
				"modes": map[string]any{"classic": true, "secure": true, "tls": true},
			},
			"censorship": map[string]any{"tls_domain": domain, "mask": true},
			"server":     map[string]any{"listeners": []any{map[string]any{"ip": "0.0.0.0"}}},
			"upstreams":  []any{map[string]any{"address": "149.154.167.51:443", "enabled": true}},
		},
	}
	s.revision = configRevision(s.config)
	s.seed()
	return s
}

func u64(v uint64) *uint64 { return &v }
func str(v string) *string { return &v }

// secret возвращает 32 hex-символа, детерминированные по имени и номеру.
func secret(name string, generation int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s/%d", name, generation)))
	return hex.EncodeToString(sum[:16])
}

func (s *server) makeLinks(name string, generation int) *Links {
	key := secret(name, generation)
	return &Links{
		Classic: []string{fmt.Sprintf("tg://proxy?server=%s&port=443&secret=%s", s.domain, key)},
		Secure:  []string{fmt.Sprintf("tg://proxy?server=%s&port=443&secret=dd%s", s.domain, key)},
		TLS:     []string{fmt.Sprintf("tg://proxy?server=%s&port=443&secret=ee%s%s", s.domain, key, s.domain)},
	}
}

func (s *server) seed() {
	now := time.Now().UTC()
	expired := now.Add(-24 * time.Hour).Format(time.RFC3339)
	soon := now.Add(30 * 24 * time.Hour).Format(time.RFC3339)

	type item struct {
		name    string
		enabled bool
		ips     []string
		conns   uint64
		total   uint64
		maxTCP  *uint64
		maxIPs  *uint64
		expires *string
		quota   *uint64
		period  *string
		up      *uint64
		down    *uint64
	}

	items := []item{
		{
			name: "router-home", enabled: true,
			ips:   []string{"192.0.2.10", "192.0.2.11"},
			conns: 12, total: 12_400_000_000,
			maxIPs: u64(5),
		},
		{
			name: "phone-main", enabled: true,
			ips:   []string{"192.0.2.24", "198.51.100.7"},
			conns: 4, total: 4_250_000_000,
			maxIPs: u64(3), expires: str(soon),
		},
		{
			name: "laptop-work", enabled: true,
			ips:   []string{"192.0.2.31"},
			conns: 2, total: 830_000_000,
			up: u64(2_000_000), down: u64(8_000_000),
		},
		{
			name: "tablet-kids", enabled: true,
			ips:   []string{"192.0.2.44"},
			conns: 1, total: 1_100_000_000,
			quota: u64(5_000_000_000), period: str("month"),
		},
		{
			name: "travel-phone", enabled: true,
			ips:   []string{"198.51.100.21", "203.0.113.9"},
			conns: 3, total: 2_600_000_000,
		},
		{
			name: "tv-livingroom", enabled: true,
			ips:   []string{"192.0.2.55"},
			conns: 6, total: 38_900_000_000,
		},
		{
			name: "test-device", enabled: false,
			ips:    []string{},
			total:  0,
			maxTCP: u64(10),
		},
		{
			name: "expired-trial", enabled: false,
			ips:     []string{"203.0.113.77"},
			total:   320_000_000,
			expires: str(expired),
		},
	}

	for _, it := range items {
		u := &User{
			Username:           it.name,
			Enabled:            it.enabled,
			InRuntime:          it.enabled,
			MaxTCPConns:        it.maxTCP,
			MaxUniqueIPs:       it.maxIPs,
			ExpirationRFC3339:  it.expires,
			DataQuotaBytes:     it.quota,
			QuotaPeriod:        it.period,
			RateLimitUpBps:     it.up,
			RateLimitDownBps:   it.down,
			CurrentConnections: it.conns,
			ActiveUniqueIPs:    uint64(len(it.ips)),
			ActiveUniqueIPList: it.ips,
			RecentUniqueIPList: it.ips,
			TotalOctets:        it.total,
			Links:              s.makeLinks(it.name, 0),
		}
		s.users[it.name] = u
		s.order = append(s.order, it.name)
	}
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /v1/capabilities", s.handleCapabilities)
	mux.HandleFunc("GET /v1/config", s.handleConfig)
	mux.HandleFunc("GET /v1/config/schema", s.handleConfigSchema)
	mux.HandleFunc("POST /v1/config/validate", s.handleConfigValidate)
	mux.HandleFunc("PATCH /v1/config", s.handleConfigPatch)
	mux.HandleFunc("GET /v1/system/reload/{id}", s.handleReloadStatus)
	mux.HandleFunc("GET /v1/users", s.handleList)
	mux.HandleFunc("POST /v1/users", s.handleCreate)
	mux.HandleFunc("GET /v1/users/{name}", s.handleGet)
	mux.HandleFunc("PATCH /v1/users/{name}", s.handlePatch)
	mux.HandleFunc("DELETE /v1/users/{name}", s.handleDelete)
	mux.HandleFunc("POST /v1/users/{name}/reset-quota", s.handleResetQuota)
	mux.HandleFunc("POST /v1/users/{name}/rotate-secret", s.handleRotate)
	mux.HandleFunc("POST /v1/users/{name}/enable", s.handleEnable)
	mux.HandleFunc("POST /v1/users/{name}/disable", s.handleDisable)
	return s.withAuth(mux)
}

func (s *server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token != "" && r.Header.Get("Authorization") != s.token {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid API token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) handleCapabilities(w http.ResponseWriter, _ *http.Request) {
	writeOK(w, map[string]any{
		"version": "3.5.17-demo",
		"api":     "v1",
		"demo":    true,
	})
}

func (s *server) handleList(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]User, 0, len(s.order))
	for _, name := range s.order {
		if u, ok := s.users[name]; ok {
			out = append(out, *u)
		}
	}
	writeOK(w, out)
}

func (s *server) handleGet(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[r.PathValue("name")]
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	writeOK(w, u)
}

func (s *server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username          string  `json:"username"`
		MaxTCPConns       *uint64 `json:"max_tcp_conns"`
		MaxUniqueIPs      *uint64 `json:"max_unique_ips"`
		ExpirationRFC3339 *string `json:"expiration_rfc3339"`
		DataQuotaBytes    *uint64 `json:"data_quota_bytes"`
		QuotaPeriod       *string `json:"quota_period"`
		RateLimitUpBps    *uint64 `json:"rate_limit_up_bps"`
		RateLimitDownBps  *uint64 `json:"rate_limit_down_bps"`
		Enabled           *bool   `json:"enabled"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	name := strings.TrimSpace(in.Username)
	if name == "" || strings.Contains(name, "/") {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid username")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[name]; exists {
		writeError(w, http.StatusConflict, "exists", "user already exists")
		return
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	u := &User{
		Username:          name,
		Enabled:           enabled,
		InRuntime:         enabled,
		MaxTCPConns:       in.MaxTCPConns,
		MaxUniqueIPs:      in.MaxUniqueIPs,
		ExpirationRFC3339: in.ExpirationRFC3339,
		DataQuotaBytes:    in.DataQuotaBytes,
		QuotaPeriod:       in.QuotaPeriod,
		RateLimitUpBps:    in.RateLimitUpBps,
		RateLimitDownBps:  in.RateLimitDownBps,
		Links:             s.makeLinks(name, 0),
	}
	s.users[name] = u
	s.order = append(s.order, name)
	writeOK(w, map[string]any{"user": u})
}

func (s *server) handlePatch(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var patch map[string]json.RawMessage
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[name]
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	for key, raw := range patch {
		switch key {
		case "max_tcp_conns":
			u.MaxTCPConns = decodePtr[uint64](raw)
		case "max_unique_ips":
			u.MaxUniqueIPs = decodePtr[uint64](raw)
		case "expiration_rfc3339":
			u.ExpirationRFC3339 = decodePtr[string](raw)
		case "data_quota_bytes":
			u.DataQuotaBytes = decodePtr[uint64](raw)
		case "quota_period":
			u.QuotaPeriod = decodePtr[string](raw)
		case "rate_limit_up_bps":
			u.RateLimitUpBps = decodePtr[uint64](raw)
		case "rate_limit_down_bps":
			u.RateLimitDownBps = decodePtr[uint64](raw)
		case "enabled":
			if v := decodePtr[bool](raw); v != nil {
				u.Enabled = *v
				u.InRuntime = *v
			}
		}
	}
	writeOK(w, u)
}

func decodePtr[T any](raw json.RawMessage) *T {
	if string(raw) == "null" {
		return nil
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	return &v
}

func (s *server) handleDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[name]; !ok {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	delete(s.users, name)
	for i, n := range s.order {
		if n == name {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
	writeOK(w, map[string]any{"deleted": name})
}

func (s *server) handleResetQuota(w http.ResponseWriter, r *http.Request) {
	s.mutate(w, r, func(u *User) { u.TotalOctets = 0 })
}

func (s *server) handleRotate(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[r.PathValue("name")]
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	s.rots++
	u.Links = s.makeLinks(u.Username, s.rots)
	writeOK(w, u)
}

func (s *server) handleEnable(w http.ResponseWriter, r *http.Request) {
	s.mutate(w, r, func(u *User) { u.Enabled = true; u.InRuntime = true })
}

func (s *server) handleDisable(w http.ResponseWriter, r *http.Request) {
	s.mutate(w, r, func(u *User) { u.Enabled = false; u.InRuntime = false })
}

func (s *server) mutate(w http.ResponseWriter, r *http.Request, fn func(*User)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[r.PathValue("name")]
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	fn(u)
	writeOK(w, u)
}

func (s *server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeEnvelope(w, s.config, s.revision)
}

func (s *server) handleConfigSchema(w http.ResponseWriter, _ *http.Request) {
	writeOK(w, map[string]any{
		"schema_version": 1,
		"fields": []map[string]any{
			{"path": "general.links.public_host", "description": "Public hostname for tg:// links", "value": map[string]any{"type": "string"}, "api": map[string]any{"config_read": true, "config_write": true}},
			{"path": "general.links.public_port", "description": "Port for tg:// links", "value": map[string]any{"type": "integer"}, "api": map[string]any{"config_read": true, "config_write": true}},
			{"path": "general.modes.tls", "description": "Enable FakeTLS mode", "value": map[string]any{"type": "boolean"}, "api": map[string]any{"config_read": true, "config_write": true}},
			{"path": "censorship.tls_domain", "description": "Cover domain for FakeTLS", "value": map[string]any{"type": "string"}, "api": map[string]any{"config_read": true, "config_write": true}},
			{"path": "censorship.mask", "description": "Mask unknown connections", "value": map[string]any{"type": "boolean"}, "api": map[string]any{"config_read": true, "config_write": true}},
			{"path": "upstreams", "description": "Direct telemt upstreams", "value": map[string]any{"type": "array", "items": map[string]any{"type": "object"}}, "api": map[string]any{"config_read": true, "config_write": true}},
			{"path": "server.port", "value": map[string]any{"type": "integer"}, "api": map[string]any{"config_read": false, "config_write": false}, "non_editable_reason": "process_owned"},
		},
	})
}

func (s *server) handleConfigValidate(w http.ResponseWriter, r *http.Request) {
	var patch map[string]any
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	if access, ok := patch["access"]; ok && access != nil {
		writeError(w, http.StatusBadRequest, "access_not_editable", "access is managed via users API")
		return
	}
	writeOK(w, map[string]any{})
}

func (s *server) handleConfigPatch(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ifMatch := r.Header.Get("If-Match"); ifMatch != "" && ifMatch != s.revision {
		writeError(w, http.StatusConflict, "revision_conflict", "revision changed")
		return
	}
	var patch map[string]any
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	mergeConfig(s.config, patch)
	s.revision = configRevision(s.config)
	s.reloadSeq++
	reloadID := s.reloadSeq
	s.reloads[reloadID] = "succeeded"
	data := map[string]any{
		"revision":      s.revision,
		"changed_paths": flattenPaths("", patch),
		"reload":        map[string]any{"reload_id": reloadID, "state": "accepted"},
	}
	writeEnvelope(w, data, s.revision)
}

func (s *server) handleReloadStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var id int64
	_, _ = fmt.Sscanf(r.PathValue("id"), "%d", &id)
	state, ok := s.reloads[id]
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "reload not found")
		return
	}
	writeOK(w, map[string]any{"reload_id": id, "state": state, "phase": state})
}

func writeEnvelope(w http.ResponseWriter, data any, revision string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "revision": revision, "data": data})
}

func configRevision(config map[string]any) string {
	raw, _ := json.Marshal(config)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func mergeConfig(dst, patch map[string]any) {
	for key, value := range patch {
		if child, ok := value.(map[string]any); ok {
			existing, _ := dst[key].(map[string]any)
			if existing == nil {
				existing = map[string]any{}
				dst[key] = existing
			}
			mergeConfig(existing, child)
			continue
		}
		dst[key] = value
	}
}

func flattenPaths(prefix string, value map[string]any) []string {
	out := []string{}
	for key, child := range value {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if nested, ok := child.(map[string]any); ok {
			out = append(out, flattenPaths(path, nested)...)
			continue
		}
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func writeOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": data})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok": false,
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
