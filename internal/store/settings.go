package store

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	settingsGlobalKey  = "settings:global"
	settingsVersionKey = "settings:version"
	require2FASinceKey = "security:require_2fa_since"
)

// ErrVersionConflict возвращается, когда настройки уже изменил другой админ.
var ErrVersionConflict = errors.New("версия настроек устарела")

// GlobalSettings возвращает переопределения и версию.
func (s *Store) GlobalSettings(ctx context.Context) (map[string]string, int64, error) {
	overrides, err := s.rdb.HGetAll(ctx, settingsGlobalKey).Result()
	if err != nil {
		return nil, 0, err
	}
	version, err := s.rdb.Get(ctx, settingsVersionKey).Int64()
	if errors.Is(err, redis.Nil) {
		version = 0
	} else if err != nil {
		return nil, 0, err
	}
	return overrides, version, nil
}

// SaveGlobalSettings атомарно применяет изменения при совпадении версии.
func (s *Store) SaveGlobalSettings(
	ctx context.Context,
	expected int64,
	set map[string]string,
	reset []string,
) (int64, error) {
	next := expected + 1
	err := s.rdb.Watch(ctx, func(tx *redis.Tx) error {
		current, err := tx.Get(ctx, settingsVersionKey).Int64()
		if errors.Is(err, redis.Nil) {
			current = 0
		} else if err != nil {
			return err
		}
		if current != expected {
			return ErrVersionConflict
		}
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			if len(set) > 0 {
				fields := make(map[string]any, len(set))
				for key, value := range set {
					fields[key] = value
				}
				pipe.HSet(ctx, settingsGlobalKey, fields)
			}
			if len(reset) > 0 {
				pipe.HDel(ctx, settingsGlobalKey, reset...)
			}
			pipe.Set(ctx, settingsVersionKey, strconv.FormatInt(next, 10), 0)
			return nil
		})
		return err
	}, settingsGlobalKey, settingsVersionKey)
	if err != nil {
		return 0, err
	}
	return next, nil
}

// Require2FASince возвращает момент включения обязательного 2FA.
func (s *Store) Require2FASince(ctx context.Context) (time.Time, bool, error) {
	raw, err := s.rdb.Get(ctx, require2FASinceKey).Result()
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

// SetRequire2FASince запоминает момент включения обязательного 2FA.
func (s *Store) SetRequire2FASince(ctx context.Context, at time.Time) error {
	return s.rdb.Set(ctx, require2FASinceKey, at.UTC().Format(time.RFC3339), 0).Err()
}

// ClearRequire2FASince снимает отметку обязательного 2FA.
func (s *Store) ClearRequire2FASince(ctx context.Context) error {
	return s.rdb.Del(ctx, require2FASinceKey).Err()
}
