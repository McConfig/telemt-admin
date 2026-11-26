package web

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/McConfig/telemt-admin/internal/settings"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
	"github.com/go-chi/chi/v5"
)

type bulkResetRequest struct {
	Names []string `json:"names"`
}

type bulkResetFailure struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // refused, unknown, not_sent
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

type bulkResetResponse struct {
	Total   int                `json:"total"`
	Reset   int                `json:"reset"`
	Refused int                `json:"refused"`
	Unknown int                `json:"unknown"`
	NotSent int                `json:"notSent"`
	Failed  []bulkResetFailure `json:"failed"`
}

func (out *bulkResetResponse) reject(name, status, code, message string) {
	out.Failed = append(out.Failed, bulkResetFailure{
		Name: name, Status: status, Code: code, Message: message,
	})
	switch status {
	case "refused":
		out.Refused++
	case "not_sent":
		out.NotSent++
	default:
		out.Unknown++
	}
}

func (s *Server) apiBulkResetQuota(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	var in bulkResetRequest
	if r.ContentLength != 0 {
		if err := decodeJSON(r, &in); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "некорректный JSON")
			return
		}
	}
	names := in.Names
	if len(names) == 0 {
		users, err := s.telemt.ListUsers(r.Context())
		if err != nil {
			s.telemtAPIError(w, r, err)
			return
		}
		for _, u := range users {
			names = append(names, u.Username)
		}
	}
	if len(names) > 1000 {
		writeError(w, http.StatusBadRequest, "bad_request", "не больше 1000 клиентов за раз")
		return
	}
	seen := map[string]bool{}
	out := bulkResetResponse{Failed: []bulkResetFailure{}}
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		out.Total++
		if name == "" {
			out.reject(name, "not_sent", "empty_name", "пустое имя")
			continue
		}
		if seen[name] {
			out.reject(name, "not_sent", "duplicate", "имя повторяется")
			continue
		}
		seen[name] = true
		if !nameRe.MatchString(name) {
			out.reject(name, "not_sent", "invalid_name", "недопустимое имя")
			continue
		}
		locked, err := s.store.TryQuotaLock(r.Context(), name, 30*time.Second)
		if err != nil {
			out.reject(name, "unknown", "store_error", "не удалось взять блокировку")
			continue
		}
		if !locked {
			out.reject(name, "refused", "quota_busy", "сброс уже выполняется")
			continue
		}
		err = s.telemt.ResetQuota(r.Context(), name)
		s.store.UnlockQuota(r.Context(), name)
		if err != nil {
			var apiErr *telemt.APIError
			if errors.As(err, &apiErr) {
				status := "unknown"
				if apiErr.Status >= 400 && apiErr.Status < 500 {
					status = "refused"
				}
				out.reject(name, status, apiErr.Code, apiErr.Message)
			} else {
				out.reject(name, "unknown", "", err.Error())
			}
			continue
		}
		if err := s.store.SetQuotaLastRun(r.Context(), name, time.Now().UTC()); err != nil {
			slog.Warn("не удалось сохранить время сброса квоты", "user", name, "err", err)
		}
		out.Reset++
	}
	target := "selected"
	if len(in.Names) == 0 {
		target = "all"
	}
	s.audit(r, "quota.reset.bulk", target, resultText(out))
	writeJSON(w, http.StatusOK, out)
}

func resultText(out bulkResetResponse) string {
	return "reset=" + strconv.Itoa(out.Reset) +
		" refused=" + strconv.Itoa(out.Refused) +
		" unknown=" + strconv.Itoa(out.Unknown) +
		" not_sent=" + strconv.Itoa(out.NotSent)
}

type quotaResetView struct {
	Mode    string `json:"mode"`
	Days    int    `json:"days,omitempty"`
	Time    string `json:"time,omitempty"`
	Enabled bool   `json:"enabled"`
	UseDays int    `json:"useDays,omitempty"`
	UseTime string `json:"useTime,omitempty"`
	Last    string `json:"last,omitempty"`
	Next    string `json:"next,omitempty"`
}

func (s *Server) apiGetQuotaReset(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if !nameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "недопустимое имя")
		return
	}
	view, err := s.quotaResetView(r, name)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) apiSetQuotaReset(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if !nameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "недопустимое имя")
		return
	}
	var in store.QuotaReset
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "некорректный JSON")
		return
	}
	if err := settings.ValidateQuotaReset(in); err != nil {
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
	if in.Mode == "inherit" {
		if err := s.store.DeleteQuotaReset(r.Context(), name); err != nil {
			s.serverErrorJSON(w, r, err)
			return
		}
	} else if err := s.store.SetQuotaReset(r.Context(), name, in); err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	s.audit(r, "user.quota_reset", name, in.Mode)
	view, err := s.quotaResetView(r, name)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) quotaResetView(r *http.Request, name string) (quotaResetView, error) {
	override, _, err := s.store.GetQuotaReset(r.Context(), name)
	if err != nil {
		return quotaResetView{}, err
	}
	globalEnabled, globalDays, globalTime := s.settings.QuotaResetPolicy(r.Context())
	enabled, days, at := settings.EffectiveQuotaReset(globalEnabled, globalDays, globalTime, override)
	view := quotaResetView{
		Mode: override.Mode, Days: override.Days, Time: override.Time,
		Enabled: enabled, UseDays: days, UseTime: at,
	}
	if last, ok, err := s.store.QuotaLastRun(r.Context(), name); err == nil && ok {
		view.Last = last.UTC().Format(time.RFC3339)
		if next := settings.NextQuotaReset(last, days, at); !next.IsZero() {
			view.Next = next.UTC().Format(time.RFC3339)
		}
	}
	return view, nil
}
