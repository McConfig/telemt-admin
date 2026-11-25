package store

import (
	"context"
	"errors"
	"strconv"

	"github.com/redis/go-redis/v9"
)

const (
	settingsGlobalKey  = "settings:global"
	settingsVersionKey = "settings:version"
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
