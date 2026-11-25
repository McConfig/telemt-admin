package settings

import (
	"net/netip"
	"regexp"
	"strings"

	"github.com/McConfig/telemt-admin/internal/store"
)

var templateNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// ValidateTemplate проверяет шаблон лимитов.
func ValidateTemplate(t store.LimitTemplate) error {
	if !templateNameRe.MatchString(t.Name) {
		return &ValidationError{Key: "name", Message: "имя: латиница, цифры, точка, дефис и подчёркивание, до 64 символов"}
	}
	if t.MaxIps != nil && *t.MaxIps > 1000 {
		return &ValidationError{Key: "maxIps", Message: "не больше 1000"}
	}
	if t.MaxTcpConns != nil && *t.MaxTcpConns > 10000 {
		return &ValidationError{Key: "maxTcpConns", Message: "не больше 10000"}
	}
	if t.DataQuotaBytes != nil && *t.DataQuotaBytes > 1<<50 {
		return &ValidationError{Key: "dataQuotaBytes", Message: "слишком большая квота"}
	}
	if t.QuotaPeriod != nil {
		switch *t.QuotaPeriod {
		case "", "daily", "weekly", "monthly":
		default:
			return &ValidationError{Key: "quotaPeriod", Message: "daily, weekly или monthly"}
		}
	}
	if t.RateLimitUpBps != nil && *t.RateLimitUpBps > 1_000_000_000_000 {
		return &ValidationError{Key: "rateLimitUpBps", Message: "слишком большая скорость"}
	}
	if t.RateLimitDownBps != nil && *t.RateLimitDownBps > 1_000_000_000_000 {
		return &ValidationError{Key: "rateLimitDownBps", Message: "слишком большая скорость"}
	}
	if t.ExpirationDays != nil && (*t.ExpirationDays < 0 || *t.ExpirationDays > 3650) {
		return &ValidationError{Key: "expirationDays", Message: "от 0 до 3650 дней"}
	}
	if len(t.SourceDeny) > 50 {
		return &ValidationError{Key: "sourceDeny", Message: "не больше 50 сетей"}
	}
	for _, raw := range t.SourceDeny {
		if _, err := netip.ParsePrefix(strings.TrimSpace(raw)); err != nil {
			return &ValidationError{Key: "sourceDeny", Message: "некорректная сеть: " + raw}
		}
	}
	if t.RiskWarnIps != nil && (*t.RiskWarnIps < 0 || *t.RiskWarnIps > 1000) {
		return &ValidationError{Key: "riskWarnIps", Message: "от 0 до 1000"}
	}
	if t.RiskHighIps != nil && (*t.RiskHighIps < 0 || *t.RiskHighIps > 1000) {
		return &ValidationError{Key: "riskHighIps", Message: "от 0 до 1000"}
	}
	if t.RiskDistinct7d != nil && (*t.RiskDistinct7d < 0 || *t.RiskDistinct7d > 10000) {
		return &ValidationError{Key: "riskDistinct7d", Message: "от 0 до 10000"}
	}
	if t.RiskWarnIps != nil && t.RiskHighIps != nil && *t.RiskWarnIps >= *t.RiskHighIps {
		return &ValidationError{Key: "riskHighIps", Message: "должен быть больше предупреждения"}
	}
	return nil
}
