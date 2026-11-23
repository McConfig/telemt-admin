package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

var (
	ErrNotFound = errors.New("не найдено")
	ErrExists   = errors.New("уже существует")
)

// Store — обёртка над Valkey.
type Store struct {
	rdb *redis.Client
}

// Open подключается к Valkey по URL.
func Open(ctx context.Context, rawURL, password string) (*Store, error) {
	opt, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("разбор VALKEY_URL: %w", err)
	}
	if password != "" {
		opt.Password = password
	}
	rdb := redis.NewClient(opt)
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("подключение к Valkey: %w", err)
	}
	return &Store{rdb: rdb}, nil
}

func (s *Store) Close() error { return s.rdb.Close() }
