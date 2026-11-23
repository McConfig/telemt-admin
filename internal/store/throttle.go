package store

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	maxLoginFailures = 10
	loginFailWindow  = 15 * time.Minute
)

func failKey(kind, value string) string { return "auth:fail:" + kind + ":" + value }

// LoginAllowed проверяет, не превышен ли лимит неудачных входов.
func (s *Store) LoginAllowed(ctx context.Context, ip, user string) (bool, error) {
	for _, k := range []string{failKey("ip", ip), failKey("user", user)} {
		n, err := s.rdb.Get(ctx, k).Int()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			return false, err
		}
		if n >= maxLoginFailures {
			return false, nil
		}
	}
	return true, nil
}

// RecordLoginFailure увеличивает счётчики неудачных входов.
func (s *Store) RecordLoginFailure(ctx context.Context, ip, user string) error {
	for _, k := range []string{failKey("ip", ip), failKey("user", user)} {
		n, err := s.rdb.Incr(ctx, k).Result()
		if err != nil {
			return err
		}
		if n == 1 {
			if err := s.rdb.Expire(ctx, k, loginFailWindow).Err(); err != nil {
				return err
			}
		}
	}
	return nil
}

// ResetLoginFailures обнуляет счётчики после успешного входа.
func (s *Store) ResetLoginFailures(ctx context.Context, ip, user string) error {
	return s.rdb.Del(ctx, failKey("ip", ip), failKey("user", user)).Err()
}
