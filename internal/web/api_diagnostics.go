package web

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
	"github.com/McConfig/telemt-admin/internal/version"
	"github.com/McConfig/telemt-admin/internal/webhook"
)

type diagTelemt struct {
	URL       string `json:"url"`
	OK        bool   `json:"ok"`
	LatencyMs int64  `json:"latencyMs"`
	Version   string `json:"version,omitempty"`
	Code      string `json:"code,omitempty"`
	Error     string `json:"error,omitempty"`
	Hint      string `json:"hint,omitempty"`
}

type diagCollector struct {
	Up          bool   `json:"up"`
	LastSuccess string `json:"lastSuccess,omitempty"`
	LastError   string `json:"lastError,omitempty"`
}

type diagGeoIP struct {
	Loaded  bool   `json:"loaded"`
	Built   string `json:"built,omitempty"`
	AgeDays int    `json:"ageDays"`
}

type diagValkey struct {
	OK        bool   `json:"ok"`
	LatencyMs int64  `json:"latencyMs"`
	Keys      int64  `json:"keys"`
	Memory    string `json:"memory,omitempty"`
	Error     string `json:"error,omitempty"`
}

type diagWebhooks struct {
	Configured bool     `json:"configured"`
	Events     []string `json:"events"`
	SecretSet  bool     `json:"secretSet"`
	LastAt     string   `json:"lastAt,omitempty"`
	LastEvent  string   `json:"lastEvent,omitempty"`
	LastStatus int      `json:"lastStatus,omitempty"`
	LastError  string   `json:"lastError,omitempty"`
	Sent       int64    `json:"sent"`
	Failed     int64    `json:"failed"`
}

type diagSecurity struct {
	Require2FA        bool     `json:"require2fa"`
	GraceDays         int      `json:"graceDays"`
	Deadline          string   `json:"deadline,omitempty"`
	DaysLeft          int      `json:"daysLeft"`
	TOTPKeySet        bool     `json:"totpKeySet"`
	AdminsWithoutTOTP []string `json:"adminsWithoutTotp"`
}

type diagnosticsResponse struct {
	Status    string        `json:"status"`
	CheckedAt string        `json:"checkedAt"`
	Version   string        `json:"version"`
	Uptime    string        `json:"uptime"`
	Telemt    diagTelemt    `json:"telemt"`
	Collector diagCollector `json:"collector"`
	GeoIP     diagGeoIP     `json:"geoip"`
	Valkey    diagValkey    `json:"valkey"`
	Webhooks  diagWebhooks  `json:"webhooks"`
	Backups   backupStatus  `json:"backups"`
	Security  diagSecurity  `json:"security"`
	Warnings  []string      `json:"warnings"`
}

func (s *Server) apiDiagnostics(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	ctx := r.Context()
	now := s.now().UTC()
	out := diagnosticsResponse{
		CheckedAt: now.Format(time.RFC3339),
		Version:   version.Version,
		Uptime:    time.Since(s.startedAt).Truncate(time.Second).String(),
		Warnings:  []string{},
	}
	hasError := false

	// Control API.
	start := time.Now()
	var caps map[string]any
	var err error
	if s.telemt != nil {
		caps, err = s.telemt.Capabilities(ctx)
	} else {
		err = fmt.Errorf("клиент Control API не создан")
	}
	out.Telemt.LatencyMs = time.Since(start).Milliseconds()
	out.Telemt.URL = s.cfg.TelemtURL
	if err != nil {
		out.Telemt.OK = false
		var apiErr *telemt.APIError
		if errors.As(err, &apiErr) {
			out.Telemt.Code = apiErr.Code
			out.Telemt.Error = apiErr.Message
			out.Telemt.Hint = telemt.Hint(apiErr.Code)
		} else {
			out.Telemt.Code = telemt.CodeUnreachable
			out.Telemt.Error = err.Error()
			out.Telemt.Hint = telemt.Hint(telemt.CodeUnreachable)
		}
		out.Warnings = append(out.Warnings, "Control API недоступен: "+out.Telemt.Error)
		hasError = true
	} else {
		out.Telemt.OK = true
		if v, ok := caps["version"]; ok {
			out.Telemt.Version = fmt.Sprint(v)
		}
	}

	// Collector.
	if s.status != nil {
		st := s.status()
		out.Collector.Up = st.Up
		out.Collector.LastError = st.LastError
		if !st.LastSuccess.IsZero() {
			out.Collector.LastSuccess = st.LastSuccess.UTC().Format(time.RFC3339)
			interval := s.settings.CollectInterval(ctx)
			if interval > 0 && now.Sub(st.LastSuccess) > 3*interval {
				out.Warnings = append(out.Warnings, "сборщик давно не обновлял данные")
			}
		}
		if !st.Up {
			out.Warnings = append(out.Warnings, "сборщик не работает")
		}
		if st.LastError != "" {
			out.Warnings = append(out.Warnings, "ошибка сборщика: "+st.LastError)
		}
		out.GeoIP.Loaded = st.GeoIPLoaded
		if st.GeoIPBuilt > 0 {
			built := time.Unix(int64(st.GeoIPBuilt), 0).UTC()
			out.GeoIP.Built = built.Format(time.RFC3339)
			out.GeoIP.AgeDays = int(now.Sub(built).Hours() / 24)
		}
	}
	if !out.GeoIP.Loaded {
		out.Warnings = append(out.Warnings, "GeoIP выключен, страна и оператор не определяются")
	} else if out.GeoIP.AgeDays > 30 {
		out.Warnings = append(out.Warnings, "базы GeoIP старше 30 дней")
	}

	// Valkey.
	start = time.Now()
	pingErr := s.store.Ping(ctx)
	out.Valkey.LatencyMs = time.Since(start).Milliseconds()
	out.Valkey.OK = pingErr == nil
	if pingErr != nil {
		out.Valkey.Error = pingErr.Error()
		out.Warnings = append(out.Warnings, "Valkey недоступен")
		hasError = true
	} else {
		out.Valkey.Keys, _ = s.store.DBSize(ctx)
		if memory, err := s.store.InfoSection(ctx, "memory"); err == nil {
			out.Valkey.Memory = memory["used_memory_human"]
		}
	}

	// Webhooks.
	_, whSecret, events, enabled := s.settings.Webhook(ctx)
	out.Webhooks.Configured = enabled
	out.Webhooks.Events = events
	out.Webhooks.SecretSet = whSecret != ""
	if enabled && whSecret == "" {
		out.Warnings = append(out.Warnings, "вебхук настроен без секрета подписи")
	}
	if provider, ok := s.notifier.(interface{ Status() webhook.Status }); ok {
		st := provider.Status()
		out.Webhooks.Sent = st.Sent
		out.Webhooks.Failed = st.Failed
		out.Webhooks.LastStatus = st.LastStatus
		out.Webhooks.LastError = st.LastError
		out.Webhooks.LastEvent = st.LastEvent
		if !st.LastAt.IsZero() {
			out.Webhooks.LastAt = st.LastAt.UTC().Format(time.RFC3339)
		}
		if st.LastError != "" {
			out.Warnings = append(out.Warnings, "последняя доставка вебхука не удалась: "+st.LastError)
		}
	}
	if !enabled {
		out.Webhooks.Events = []string{}
	}

	// Backups.
	if status, err := s.buildBackupStatus(ctx); err == nil {
		out.Backups = status
		if !status.Host.Available {
			out.Warnings = append(out.Warnings, "статус хостовых бэкапов недоступен")
		} else if status.Host.LastRun != "" {
			if last, err := time.Parse(time.RFC3339, status.Host.LastRun); err == nil &&
				now.Sub(last) > 48*time.Hour {
				out.Warnings = append(out.Warnings, "последний хостовый бэкап старше 48 часов")
			}
		}
	}
	if s.cfg.BackupConfig == "" {
		out.Warnings = append(out.Warnings, "TGADMIN_BACKUP_CONFIG не задан, расписание не применяется")
	}

	// Security.
	require2FA, deadline, daysLeft := s.settings.Require2FA(ctx)
	out.Security.Require2FA = require2FA
	out.Security.DaysLeft = daysLeft
	out.Security.TOTPKeySet = len(s.cfg.TOTPKey) == 32
	if require2FA {
		out.Security.Deadline = deadline.UTC().Format(time.RFC3339)
		out.Security.GraceDays = s.settings.Require2FAGraceDays(ctx)
	}
	if users, err := s.store.ListUsers(ctx); err == nil {
		for _, u := range users {
			if u.Role == store.RoleAdmin && !u.TOTPEnabled {
				out.Security.AdminsWithoutTOTP = append(out.Security.AdminsWithoutTOTP, u.Name)
			}
		}
	}
	if out.Security.AdminsWithoutTOTP == nil {
		out.Security.AdminsWithoutTOTP = []string{}
	}
	if require2FA && !out.Security.TOTPKeySet {
		out.Warnings = append(out.Warnings, "2FA обязательна, но TGADMIN_TOTP_KEY не задан")
		hasError = true
	}
	if require2FA && len(out.Security.AdminsWithoutTOTP) > 0 {
		if daysLeft <= 3 {
			out.Warnings = append(out.Warnings, fmt.Sprintf(
				"без 2FA осталось %d дн. у администраторов: %v", daysLeft, out.Security.AdminsWithoutTOTP))
		}
	}

	switch {
	case hasError:
		out.Status = "error"
	case len(out.Warnings) > 0:
		out.Status = "warn"
	default:
		out.Status = "ok"
	}
	writeJSON(w, http.StatusOK, out)
}
