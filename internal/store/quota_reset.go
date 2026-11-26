package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// QuotaReset — расписание сброса квоты на клиента.
type QuotaReset struct {
	Mode string `json:"mode"` // inherit, off, custom
	Days int    `json:"days,omitempty"`
	Time string `json:"time,omitempty"`
}

func quotaResetKey(name string) string { return "quota:reset:" + name }
func quotaLastKey(name string) string  { return "quota:last:" + name }

// GetQuotaReset читает переопределение расписания.
func (s *Store) GetQuotaReset(ctx context.Context, name string) (QuotaReset, bool, error) {
	raw, err := s.rdb.Get(ctx, quotaResetKey(name)).Result()
	if errors.Is(err, redis.Nil) {
		return QuotaReset{Mode: "inherit"}, false, nil
	}
	if err != nil {
		return QuotaReset{}, false, err
	}
	var q QuotaReset
	if err := json.Unmarshal([]byte(raw), &q); err != nil {
		return QuotaReset{Mode: "inherit"}, false, nil
	}
	if q.Mode == "" {
		q.Mode = "inherit"
	}
	return q, true, nil
}

// SetQuotaReset сохраняет переопределение расписания.
func (s *Store) SetQuotaReset(ctx context.Context, name string, q QuotaReset) error {
	raw, err := json.Marshal(q)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, quotaResetKey(name), string(raw), 0).Err()
}

// DeleteQuotaReset возвращает наследование глобального расписания.
func (s *Store) DeleteQuotaReset(ctx context.Context, name string) error {
	return s.rdb.Del(ctx, quotaResetKey(name)).Err()
}

// QuotaLastRun возвращает время последнего автоматического сброса.
func (s *Store) QuotaLastRun(ctx context.Context, name string) (time.Time, bool, error) {
	raw, err := s.rdb.Get(ctx, quotaLastKey(name)).Result()
	if errors.Is(err, redis.Nil) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false, nil
	}
	return t, true, nil
}

// SetQuotaLastRun запоминает время последнего автоматического сброса.
func (s *Store) SetQuotaLastRun(ctx context.Context, name string, at time.Time) error {
	return s.rdb.Set(ctx, quotaLastKey(name), at.UTC().Format(time.RFC3339), 0).Err()
}

func quotaLockKey(name string) string { return "quota:lock:" + name }

// TryQuotaLock берёт блокировку сброса квоты на время ttl.
func (s *Store) TryQuotaLock(ctx context.Context, name string, ttl time.Duration) (bool, error) {
	return s.rdb.SetNX(ctx, quotaLockKey(name), "1", ttl).Result()
}

// UnlockQuota снимает блокировку сброса квоты.
func (s *Store) UnlockQuota(ctx context.Context, name string) {
	_ = s.rdb.Del(ctx, quotaLockKey(name)).Err()
}
