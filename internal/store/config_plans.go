package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// ConfigPlan — временный план импорта конфигурации.
type ConfigPlan struct {
	ID        string    `json:"id"`
	Owner     string    `json:"owner"`
	Session   string    `json:"session"`
	Payload   []byte    `json:"payload"`
	CreatedAt time.Time `json:"createdAt"`
}

func configPlanKey(id string) string { return "config:plan:" + id }

var takeConfigPlanScript = redis.NewScript(`
local raw = redis.call('GET', KEYS[1])
if not raw then return nil end
redis.call('DEL', KEYS[1])
return raw
`)

// PutConfigPlan сохраняет план с TTL.
func (s *Store) PutConfigPlan(ctx context.Context, p ConfigPlan, ttl time.Duration) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, configPlanKey(p.ID), raw, ttl).Err()
}

// GetConfigPlan читает план без удаления.
func (s *Store) GetConfigPlan(ctx context.Context, id string) (*ConfigPlan, error) {
	raw, err := s.rdb.Get(ctx, configPlanKey(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var p ConfigPlan
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, ErrNotFound
	}
	return &p, nil
}

// TakeConfigPlan атомарно читает и удаляет план.
func (s *Store) TakeConfigPlan(ctx context.Context, id string) (*ConfigPlan, error) {
	text, err := takeConfigPlanScript.Run(ctx, s.rdb, []string{configPlanKey(id)}).Text()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var p ConfigPlan
	if err := json.Unmarshal([]byte(text), &p); err != nil {
		return nil, ErrNotFound
	}
	return &p, nil
}
