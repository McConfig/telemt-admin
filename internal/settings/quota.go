package settings

import (
	"time"

	"github.com/McConfig/telemt-admin/internal/store"
)

// ValidateQuotaReset проверяет расписание сброса квоты.
func ValidateQuotaReset(q store.QuotaReset) error {
	switch q.Mode {
	case "inherit", "off":
		return nil
	case "custom":
	default:
		return &ValidationError{Key: "mode", Message: "режим: inherit, off или custom"}
	}
	if q.Days < 1 || q.Days > 365 {
		return &ValidationError{Key: "days", Message: "интервал от 1 до 365 дней"}
	}
	if !scheduleRe.MatchString(q.Time) {
		return &ValidationError{Key: "time", Message: "время в формате ЧЧ:ММ"}
	}
	return nil
}

// EffectiveQuotaReset возвращает действующие параметры расписания.
func EffectiveQuotaReset(global bool, globalDays int, globalTime string, q store.QuotaReset) (bool, int, string) {
	switch q.Mode {
	case "off":
		return false, 0, ""
	case "custom":
		return true, q.Days, q.Time
	default:
		return global, globalDays, globalTime
	}
}

// NextQuotaReset считает следующий запуск после last.
func NextQuotaReset(last time.Time, days int, at string) time.Time {
	if last.IsZero() || days <= 0 {
		return time.Time{}
	}
	t, err := time.Parse("15:04", at)
	if err != nil {
		return time.Time{}
	}
	local := last.In(time.Local)
	next := local.AddDate(0, 0, days)
	return time.Date(next.Year(), next.Month(), next.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
}

// QuotaResetDue сообщает, пора ли выполнять сброс.
func QuotaResetDue(last time.Time, days int, at string, now time.Time) bool {
	next := NextQuotaReset(last, days, at)
	return !next.IsZero() && !now.Before(next)
}
