package store

import (
	"context"
	"strings"
)

// InfoSection читает INFO Valkey и возвращает пары ключ-значение.
func (s *Store) InfoSection(ctx context.Context, section string) (map[string]string, error) {
	raw, err := s.rdb.Info(ctx, section).Result()
	if err != nil {
		// miniredis и старые сборки могут не поддерживать секции INFO.
		raw, err = s.rdb.Info(ctx).Result()
		if err != nil {
			return nil, err
		}
	}
	out := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		out[key] = value
	}
	return out, nil
}

// DBSize возвращает число ключей.
func (s *Store) DBSize(ctx context.Context) (int64, error) {
	return s.rdb.DBSize(ctx).Result()
}

// AuditLen возвращает длину потока аудита.
func (s *Store) AuditLen(ctx context.Context) (int64, error) {
	return s.rdb.XLen(ctx, auditStream).Result()
}

// BGSAVE запускает фоновое сохранение снимка Valkey.
func (s *Store) BGSAVE(ctx context.Context) error {
	_, err := s.rdb.Do(ctx, "BGSAVE").Result()
	return err
}
