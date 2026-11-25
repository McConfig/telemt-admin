package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/McConfig/telemt-admin/internal/settings"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/version"
	"github.com/go-chi/chi/v5"
)

type settingsRequest struct {
	Version int64                      `json:"version"`
	Values  map[string]json.RawMessage `json:"values"`
	Reset   []string                   `json:"reset"`
}

func (s *Server) apiGetSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, s.settings.Catalog(r.Context()))
}

func (s *Server) apiUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	var in settingsRequest
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "некорректный JSON")
		return
	}
	catalog, changes, err := s.settings.Save(r.Context(), in.Version, in.Values, in.Reset)
	if err != nil {
		var invalid *settings.ValidationError
		switch {
		case errors.As(err, &invalid):
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{"code": "bad_request", "message": invalid.Message, "key": invalid.Key},
			})
			return
		case errors.Is(err, store.ErrVersionConflict):
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":   map[string]string{"code": "conflict", "message": "настройки уже изменил другой администратор"},
				"catalog": s.settings.Catalog(r.Context()),
			})
			return
		default:
			s.serverErrorJSON(w, r, err)
			return
		}
	}
	for _, change := range changes {
		s.audit(r, "settings.update", change.Key, change.Old+" -> "+change.New)
	}
	writeJSON(w, http.StatusOK, catalog)
}

type templatesResponse struct {
	Default   string                `json:"default"`
	Templates []store.LimitTemplate `json:"templates"`
}

func (s *Server) apiListTemplates(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	templates, err := s.store.ListTemplates(r.Context())
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	def, err := s.store.DefaultTemplate(r.Context())
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, templatesResponse{Default: def, Templates: templates})
}

func (s *Server) apiSaveTemplate(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	var t store.LimitTemplate
	if err := decodeJSON(r, &t); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "некорректный JSON")
		return
	}
	if name := chi.URLParam(r, "name"); name != "" {
		t.Name = name
	}
	if err := settings.ValidateTemplate(t); err != nil {
		var invalid *settings.ValidationError
		if errors.As(err, &invalid) {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{"code": "bad_request", "message": invalid.Message, "key": invalid.Key},
			})
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := s.store.SaveTemplate(r.Context(), t); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	s.audit(r, "settings.template.save", t.Name, "ok")
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) apiDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "недопустимое имя")
		return
	}
	if err := s.store.DeleteTemplate(r.Context(), name); err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	s.audit(r, "settings.template.delete", name, "ok")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiSetDefaultTemplate(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &in); err != nil || strings.TrimSpace(in.Name) == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "нужно имя шаблона")
		return
	}
	if _, err := s.store.GetTemplate(r.Context(), in.Name); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "шаблон не найден")
		return
	}
	if err := s.store.SetDefaultTemplate(r.Context(), in.Name); err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	s.audit(r, "settings.template.default", in.Name, "ok")
	writeJSON(w, http.StatusOK, map[string]string{"default": in.Name})
}

type backupValkey struct {
	LastSave       string `json:"lastSave,omitempty"`
	SaveInProgress bool   `json:"saveInProgress"`
	LastStatus     string `json:"lastStatus,omitempty"`
	Keys           int64  `json:"keys"`
	Memory         string `json:"memory,omitempty"`
}

type backupHost struct {
	Available   bool   `json:"available"`
	LastRun     string `json:"lastRun,omitempty"`
	Result      string `json:"result,omitempty"`
	Size        int64  `json:"size,omitempty"`
	KeepDays    int    `json:"keepDays,omitempty"`
	RestoreTest bool   `json:"restoreTest,omitempty"`
}

func (s *Server) apiBackups(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	persistence, err := s.store.InfoSection(r.Context(), "persistence")
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	memory, _ := s.store.InfoSection(r.Context(), "memory")
	keys, _ := s.store.DBSize(r.Context())
	out := struct {
		Valkey backupValkey `json:"valkey"`
		Host   backupHost   `json:"host"`
	}{}
	out.Valkey = backupValkey{
		SaveInProgress: persistence["rdb_bgsave_in_progress"] == "1",
		LastStatus:     persistence["rdb_last_bgsave_status"],
		Keys:           keys,
		Memory:         memory["used_memory_human"],
	}
	if raw := persistence["rdb_last_save_time"]; raw != "" {
		if unix, err := strconv.ParseInt(raw, 10, 64); err == nil {
			out.Valkey.LastSave = time.Unix(unix, 0).UTC().Format(time.RFC3339)
		}
	}
	out.Host = readHostBackup(s.cfg.BackupStatus)
	writeJSON(w, http.StatusOK, out)
}

func readHostBackup(path string) backupHost {
	if strings.TrimSpace(path) == "" {
		return backupHost{}
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- путь задаёт оператор через env.
	if err != nil {
		return backupHost{}
	}
	var host backupHost
	if err := json.Unmarshal(raw, &host); err != nil {
		return backupHost{}
	}
	host.Available = true
	return host
}

func (s *Server) apiBackupSnapshot(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	if err := s.store.BGSAVE(r.Context()); err != nil {
		writeError(w, http.StatusBadGateway, "backup_error", "не удалось запустить BGSAVE")
		return
	}
	s.audit(r, "settings.backup", "snapshot", "ok")
	writeJSON(w, http.StatusAccepted, map[string]bool{"started": true})
}

type systemCounts struct {
	UIUsers     int64 `json:"uiUsers"`
	AuditEvents int64 `json:"auditEvents"`
	Templates   int64 `json:"templates"`
}

func (s *Server) apiSystem(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	memory, _ := s.store.InfoSection(r.Context(), "memory")
	keys, _ := s.store.DBSize(r.Context())
	users, _ := s.store.CountUsers(r.Context())
	auditEvents, _ := s.store.AuditLen(r.Context())
	templates, _ := s.store.TemplatesCount(r.Context())
	out := struct {
		Version   string       `json:"version"`
		Uptime    string       `json:"uptime"`
		Collector any          `json:"collector"`
		GeoIP     any          `json:"geoip"`
		Valkey    backupValkey `json:"valkey"`
		Counts    systemCounts `json:"counts"`
	}{
		Version: version.Version,
		Uptime:  time.Since(s.startedAt).Truncate(time.Second).String(),
		Valkey:  backupValkey{Keys: keys, Memory: memory["used_memory_human"]},
		Counts:  systemCounts{UIUsers: users, AuditEvents: auditEvents, Templates: templates},
	}
	if s.status != nil {
		st := s.status()
		out.Collector = map[string]any{
			"up": st.Up, "lastSuccess": st.LastSuccess, "lastError": st.LastError,
		}
		out.GeoIP = map[string]any{
			"loaded": st.GeoIPLoaded, "built": st.GeoIPBuilt,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) apiSettingsHistory(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	events, err := s.store.RecentAudit(r.Context(), 1000)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	out := make([]store.AuditEvent, 0, limit)
	for _, event := range events {
		if !strings.HasPrefix(event.Action, "settings.") {
			continue
		}
		out = append(out, event)
		if len(out) >= limit {
			break
		}
	}
	writeJSON(w, http.StatusOK, out)
}
