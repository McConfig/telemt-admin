package web

import (
	"context"
	"net/http"

	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/go-chi/chi/v5"
)

const (
	maxRiskIPs      = 1000
	maxRiskDistinct = 10000
)

// apiRisk — переопределения порогов риска. null означает «как глобально».
type apiRisk struct {
	WarnIPs      *int  `json:"warnIps"`
	HighIPs      *int  `json:"highIps"`
	Distinct7d   *int  `json:"distinct7d"`
	NewCountries *bool `json:"newCountries"`
}

// apiRiskDefaults — глобальные значения из окружения.
type apiRiskDefaults struct {
	WarnIPs      int  `json:"warnIps"`
	HighIPs      int  `json:"highIps"`
	Distinct7d   int  `json:"distinct7d"`
	NewCountries bool `json:"newCountries"`
}

func riskOverrides(r store.Risk) apiRisk {
	return apiRisk{
		WarnIPs: r.WarnIPs, HighIPs: r.HighIPs,
		Distinct7d: r.Distinct7d, NewCountries: r.NewCountries,
	}
}

func riskDefaults(warn, high, distinct int, newCountries bool) apiRiskDefaults {
	return apiRiskDefaults{
		WarnIPs: warn, HighIPs: high, Distinct7d: distinct,
		NewCountries: newCountries,
	}
}

func (s *Server) apiSetRisk(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if !nameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "недопустимое имя")
		return
	}
	var body apiRisk
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "некорректный JSON")
		return
	}
	risk, message := s.normalizeRisk(r.Context(), body)
	if message != "" {
		writeError(w, http.StatusBadRequest, "bad_request", message)
		return
	}
	if err := s.store.SetRisk(r.Context(), name, risk); err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	s.audit(r, "user.risk", name, "ok")
	warn, high, distinct, newCountries := s.riskValues(r.Context(), risk)
	writeJSON(w, http.StatusOK, map[string]any{
		"riskThresholds": riskOverrides(risk),
		"riskDefaults":   riskDefaults(warn, high, distinct, newCountries),
	})
}

// normalizeRisk проверяет значения и возвращает структуру для хранения.
// Пустые поля означают наследование глобального значения.
func (s *Server) normalizeRisk(ctx context.Context, in apiRisk) (store.Risk, string) {
	r := store.Risk{
		WarnIPs: in.WarnIPs, HighIPs: in.HighIPs,
		Distinct7d: in.Distinct7d, NewCountries: in.NewCountries,
	}
	if in.WarnIPs != nil && (*in.WarnIPs < 0 || *in.WarnIPs > maxRiskIPs) {
		return r, "порог предупреждения должен быть от 0 до 1000"
	}
	if in.HighIPs != nil && (*in.HighIPs < 0 || *in.HighIPs > maxRiskIPs) {
		return r, "порог высокого риска должен быть от 0 до 1000"
	}
	if in.Distinct7d != nil && (*in.Distinct7d < 0 || *in.Distinct7d > maxRiskDistinct) {
		return r, "порог адресов за 7 дней должен быть от 0 до 10000"
	}
	warn, high, _, _ := s.riskValues(ctx, r)
	if warn >= high {
		return r, "порог предупреждения должен быть меньше порога высокого риска"
	}
	return r, ""
}

// riskValues возвращает действующие пороги: переопределение или глобальное.
func (s *Server) riskValues(ctx context.Context, r store.Risk) (warn, high, distinct int, newCountries bool) {
	warn, high, distinct, newCountries, _ = s.settings.RiskValues(ctx)
	if r.WarnIPs != nil {
		warn = *r.WarnIPs
	}
	if r.HighIPs != nil {
		high = *r.HighIPs
	}
	if r.Distinct7d != nil {
		distinct = *r.Distinct7d
	}
	if r.NewCountries != nil {
		newCountries = *r.NewCountries
	}
	return warn, high, distinct, newCountries
}
