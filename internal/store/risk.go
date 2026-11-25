package store

import (
	"context"
	"strconv"
)

// Risk — переопределения порогов риска для одного клиента.
//
// nil означает «использовать глобальное значение из конфигурации».
// Нулевой указатель на ноль — это осознанное значение, а не отсутствие:
// например, WarnIPs = 0 запрещает предупреждение по числу адресов.
type Risk struct {
	WarnIPs      *int
	HighIPs      *int
	Distinct7d   *int
	NewCountries *bool
}

// Empty сообщает, что переопределений нет.
func (r Risk) Empty() bool {
	return r.WarnIPs == nil && r.HighIPs == nil && r.Distinct7d == nil && r.NewCountries == nil
}

func riskKey(user string) string { return "risk:user:" + user }

// GetRisk читает переопределения. Второй результат — есть ли запись.
func (s *Store) GetRisk(ctx context.Context, user string) (Risk, bool, error) {
	m, err := s.rdb.HGetAll(ctx, riskKey(user)).Result()
	if err != nil {
		return Risk{}, false, err
	}
	if len(m) == 0 {
		return Risk{}, false, nil
	}
	var r Risk
	if raw, ok := m["warn_ips"]; ok {
		if v, err := strconv.Atoi(raw); err == nil {
			r.WarnIPs = &v
		}
	}
	if raw, ok := m["high_ips"]; ok {
		if v, err := strconv.Atoi(raw); err == nil {
			r.HighIPs = &v
		}
	}
	if raw, ok := m["distinct_7d"]; ok {
		if v, err := strconv.Atoi(raw); err == nil {
			r.Distinct7d = &v
		}
	}
	if raw, ok := m["new_countries"]; ok {
		v := raw == "1"
		r.NewCountries = &v
	}
	return r, true, nil
}

// SetRisk заменяет переопределения. Пустая структура удаляет запись.
func (s *Store) SetRisk(ctx context.Context, user string, r Risk) error {
	if r.Empty() {
		return s.DeleteRisk(ctx, user)
	}
	fields := map[string]any{}
	if r.WarnIPs != nil {
		fields["warn_ips"] = *r.WarnIPs
	}
	if r.HighIPs != nil {
		fields["high_ips"] = *r.HighIPs
	}
	if r.Distinct7d != nil {
		fields["distinct_7d"] = *r.Distinct7d
	}
	if r.NewCountries != nil {
		value := "0"
		if *r.NewCountries {
			value = "1"
		}
		fields["new_countries"] = value
	}
	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, riskKey(user))
	pipe.HSet(ctx, riskKey(user), fields)
	_, err := pipe.Exec(ctx)
	return err
}

// DeleteRisk убирает переопределения: клиент снова наследует глобальные.
func (s *Store) DeleteRisk(ctx context.Context, user string) error {
	return s.rdb.Del(ctx, riskKey(user)).Err()
}
