package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrNotFound = errors.New("не найдено")
	ErrExists   = errors.New("уже существует")
)

// Store — обёртка над Valkey.
type Store struct {
	rdb       *redis.Client
	retention time.Duration
	home      atomic.Value // string, домашняя страна для истории
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
	st := &Store{rdb: rdb, retention: 30 * 24 * time.Hour}
	st.home.Store("RU")
	return st, nil
}

// SetHomeCountry задаёт домашнюю страну для новых наблюдений.
func (s *Store) SetHomeCountry(code string) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		code = "RU"
	}
	s.home.Store(code)
}

// HomeCountry возвращает домашнюю страну.
func (s *Store) HomeCountry() string {
	if v, ok := s.home.Load().(string); ok && v != "" {
		return v
	}
	return "RU"
}

// SetRetention задаёт срок хранения истории IP.
func (s *Store) SetRetention(d time.Duration) {
	if d > 0 {
		s.retention = d
	}
}

func (s *Store) Close() error { return s.rdb.Close() }
