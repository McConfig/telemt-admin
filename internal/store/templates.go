package store

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/redis/go-redis/v9"
)

const (
	templatesKey       = "limit:templates"
	templateDefaultKey = "limit:template:default"
	templatesMaxCount  = 50
)

// ErrTemplateNotFound — шаблон не найден.
var ErrTemplateNotFound = errors.New("шаблон не найден")

// LimitTemplate — именованный набор лимитов и порогов риска.
type LimitTemplate struct {
	Name             string   `json:"name"`
	MaxIps           *uint64  `json:"maxIps,omitempty"`
	MaxTcpConns      *uint64  `json:"maxTcpConns,omitempty"`
	DataQuotaBytes   *uint64  `json:"dataQuotaBytes,omitempty"`
	QuotaPeriod      *string  `json:"quotaPeriod,omitempty"`
	RateLimitUpBps   *uint64  `json:"rateLimitUpBps,omitempty"`
	RateLimitDownBps *uint64  `json:"rateLimitDownBps,omitempty"`
	ExpirationDays   *int     `json:"expirationDays,omitempty"`
	SourceDeny       []string `json:"sourceDeny,omitempty"`
	RiskWarnIps      *int     `json:"riskWarnIps,omitempty"`
	RiskHighIps      *int     `json:"riskHighIps,omitempty"`
	RiskDistinct7d   *int     `json:"riskDistinct7d,omitempty"`
	RiskNewCountries *bool    `json:"riskNewCountries,omitempty"`
}

// ListTemplates возвращает шаблоны по алфавиту.
func (s *Store) ListTemplates(ctx context.Context) ([]LimitTemplate, error) {
	raw, err := s.rdb.HGetAll(ctx, templatesKey).Result()
	if err != nil {
		return nil, err
	}
	out := make([]LimitTemplate, 0, len(raw))
	for name, value := range raw {
		var t LimitTemplate
		if err := json.Unmarshal([]byte(value), &t); err != nil {
			continue
		}
		t.Name = name
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// GetTemplate читает шаблон.
func (s *Store) GetTemplate(ctx context.Context, name string) (LimitTemplate, error) {
	raw, err := s.rdb.HGet(ctx, templatesKey, name).Result()
	if errors.Is(err, redis.Nil) {
		return LimitTemplate{}, ErrTemplateNotFound
	}
	if err != nil {
		return LimitTemplate{}, err
	}
	var t LimitTemplate
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		return LimitTemplate{}, err
	}
	t.Name = name
	return t, nil
}

// SaveTemplate создаёт или заменяет шаблон.
func (s *Store) SaveTemplate(ctx context.Context, t LimitTemplate) error {
	exists, err := s.rdb.HExists(ctx, templatesKey, t.Name).Result()
	if err != nil {
		return err
	}
	if !exists {
		count, err := s.rdb.HLen(ctx, templatesKey).Result()
		if err != nil {
			return err
		}
		if count >= templatesMaxCount {
			return errors.New("слишком много шаблонов")
		}
	}
	raw, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return s.rdb.HSet(ctx, templatesKey, t.Name, string(raw)).Err()
}

// DeleteTemplate удаляет шаблон и снимает назначение по умолчанию.
func (s *Store) DeleteTemplate(ctx context.Context, name string) error {
	def, _ := s.DefaultTemplate(ctx)
	pipe := s.rdb.TxPipeline()
	pipe.HDel(ctx, templatesKey, name)
	if def == name {
		pipe.Del(ctx, templateDefaultKey)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// DefaultTemplate возвращает имя шаблона по умолчанию.
func (s *Store) DefaultTemplate(ctx context.Context) (string, error) {
	name, err := s.rdb.Get(ctx, templateDefaultKey).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	return name, err
}

// SetDefaultTemplate назначает шаблон по умолчанию.
func (s *Store) SetDefaultTemplate(ctx context.Context, name string) error {
	return s.rdb.Set(ctx, templateDefaultKey, name, 0).Err()
}

// ClearDefaultTemplate снимает назначение.
func (s *Store) ClearDefaultTemplate(ctx context.Context) error {
	return s.rdb.Del(ctx, templateDefaultKey).Err()
}

// TemplatesCount возвращает число шаблонов.
func (s *Store) TemplatesCount(ctx context.Context) (int64, error) {
	return s.rdb.HLen(ctx, templatesKey).Result()
}
