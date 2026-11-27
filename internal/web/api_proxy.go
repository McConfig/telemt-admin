package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/McConfig/telemt-admin/internal/telemt"
	"github.com/go-chi/chi/v5"
)

func (s *Server) apiProxySchema(w http.ResponseWriter, r *http.Request) {
	if s.requireConfigAdmin(w, r) == nil {
		return
	}
	if s.telemt == nil {
		writeErrorHint(w, http.StatusServiceUnavailable, "proxy_unavailable", "Control API telemt недоступен", "проверьте TELEMT_API_URL")
		return
	}
	s.proxySchemaMu.Lock()
	cached := s.proxySchema
	fresh := cached != nil && s.now().Sub(s.proxySchemaAt) < 5*time.Minute
	s.proxySchemaMu.Unlock()
	if !fresh {
		schema, err := s.telemt.GetConfigSchema(r.Context())
		if err != nil {
			s.proxyError(w, r, err)
			return
		}
		s.proxySchemaMu.Lock()
		s.proxySchema, s.proxySchemaAt = schema, s.now()
		s.proxySchemaMu.Unlock()
		cached = schema
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": cached.Version, "fields": cached.Fields})
}

func (s *Server) apiProxyConfig(w http.ResponseWriter, r *http.Request) {
	if s.requireConfigAdmin(w, r) == nil {
		return
	}
	if s.telemt == nil {
		writeErrorHint(w, http.StatusServiceUnavailable, "proxy_unavailable", "Control API telemt недоступен", "проверьте TELEMT_API_URL")
		return
	}
	cfg, err := s.telemt.GetConfig(r.Context())
	if err != nil {
		s.proxyError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"revision": cfg.Revision, "config": cfg.Config})
}

func (s *Server) apiProxyValidate(w http.ResponseWriter, r *http.Request) {
	if s.requireConfigAdmin(w, r) == nil {
		return
	}
	var in struct {
		Patch map[string]any `json:"patch"`
	}
	if err := decodeJSON(r, &in); err != nil || len(in.Patch) == 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "нужен непустой patch")
		return
	}
	if err := s.telemt.ValidateConfig(r.Context(), in.Patch); err != nil {
		s.proxyError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "changedPaths": flattenPatch(in.Patch)})
}

var dangerousProxyPrefixes = []string{
	"censorship.tls_domain", "general.modes.", "server.listeners",
	"general.links.public_host", "general.links.public_port",
}

func (s *Server) apiProxyApply(w http.ResponseWriter, r *http.Request) {
	sess := s.requireConfigAdmin(w, r)
	if sess == nil {
		return
	}
	var in struct {
		Patch   map[string]any `json:"patch"`
		Confirm bool           `json:"confirm"`
	}
	if err := decodeJSON(r, &in); err != nil || len(in.Patch) == 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "нужен непустой patch")
		return
	}
	paths := flattenPatch(in.Patch)
	if !in.Confirm && hasDangerousPath(paths) {
		writeError(w, http.StatusBadRequest, "confirm_required", "подтвердите опасные изменения: "+strings.Join(paths, ", "))
		return
	}
	if err := s.telemt.ValidateConfig(r.Context(), in.Patch); err != nil {
		s.proxyError(w, r, err)
		return
	}
	cfg, err := s.telemt.GetConfig(r.Context())
	if err != nil {
		s.proxyError(w, r, err)
		return
	}
	res, err := s.telemt.PatchConfig(r.Context(), in.Patch, cfg.Revision, true)
	if err != nil {
		s.proxyError(w, r, err)
		return
	}
	s.audit(r, "proxy.update", strings.Join(paths, ","), "ok")
	if res.ReloadID != 0 {
		s.audit(r, "proxy.reload", strconv.FormatInt(res.ReloadID, 10), "accepted")
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"revision": res.Revision, "changedPaths": res.ChangedPaths,
		"reloadId": res.ReloadID, "runtimeReloadRequired": res.RuntimeReloadRequired,
		"processRestartRequired": res.ProcessRestartRequired,
		"deferredProcessFields":  res.DeferredProcessFields,
	})
}

func (s *Server) apiProxyReload(w http.ResponseWriter, r *http.Request) {
	if s.requireConfigAdmin(w, r) == nil {
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный id перезагрузки")
		return
	}
	status, err := s.telemt.ReloadStatus(r.Context(), id)
	if err != nil {
		s.proxyError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) proxyError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *telemt.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case telemt.CodeConflict:
			writeError(w, http.StatusConflict, "proxy_conflict", "конфиг изменился, обновите страницу")
			return
		case telemt.CodeBadRequest:
			writeError(w, http.StatusBadRequest, "proxy_invalid", apiErr.Message)
			return
		case telemt.CodeUnavailable, telemt.CodeUnreachable, telemt.CodeTimeout:
			writeError(w, http.StatusServiceUnavailable, "proxy_unavailable", "Control API telemt недоступен")
			return
		}
	}
	s.telemtAPIError(w, r, err)
}

func flattenPatch(patch map[string]any) []string {
	out := []string{}
	var walk func(prefix string, m map[string]any)
	walk = func(prefix string, m map[string]any) {
		for key, value := range m {
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			if child, ok := value.(map[string]any); ok {
				walk(path, child)
				continue
			}
			out = append(out, path)
		}
	}
	walk("", patch)
	return out
}

func hasDangerousPath(paths []string) bool {
	for _, path := range paths {
		for _, prefix := range dangerousProxyPrefixes {
			if path == prefix || strings.HasPrefix(path, prefix) {
				return true
			}
		}
	}
	return false
}
