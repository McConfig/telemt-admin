package store

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// ServiceScope — уровень прав сервисного аккаунта.
type ServiceScope string

const (
	ScopeRead  ServiceScope = "read"
	ScopeWrite ServiceScope = "write"
	ScopeAdmin ServiceScope = "admin"

	// MaxServiceAccounts — максимум сервисных аккаунтов.
	MaxServiceAccounts = 50
)

// ServiceAccount — машинная учётная запись без пароля.
type ServiceAccount struct {
	Name        string
	Description string
	Scope       ServiceScope
	Disabled    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   string
}

// ServiceUpdate — необязательные изменения сервисного аккаунта.
type ServiceUpdate struct {
	Description *string
	Scope       *ServiceScope
	Disabled    *bool
}

func servicesIndex() string               { return "auth:services" }
func serviceKey(name string) string       { return "auth:service:" + name }
func serviceTokensKey(name string) string { return "auth:service_tokens:" + name }
func tokenKey(id string) string           { return "auth:token:" + id }

var createServiceScript = redis.NewScript(`
local count = redis.call('SCARD', KEYS[1])
if count >= tonumber(ARGV[1]) then return -1 end
if redis.call('HSETNX', KEYS[2], 'created_at', ARGV[2]) == 0 then return 0 end
redis.call('HSET', KEYS[2],
  'description', ARGV[3], 'scope', ARGV[4], 'disabled', ARGV[5],
  'updated_at', ARGV[6], 'created_by', ARGV[7])
redis.call('SADD', KEYS[1], ARGV[8])
return 1
`)

// CreateService создаёт сервисный аккаунт.
func (s *Store) CreateService(ctx context.Context, svc ServiceAccount) error {
	now := time.Now().UTC()
	if svc.CreatedAt.IsZero() {
		svc.CreatedAt = now
	}
	if svc.UpdatedAt.IsZero() {
		svc.UpdatedAt = svc.CreatedAt
	}
	res, err := createServiceScript.Run(ctx, s.rdb,
		[]string{servicesIndex(), serviceKey(svc.Name)},
		MaxServiceAccounts, svc.CreatedAt.Format(time.RFC3339),
		svc.Description, string(svc.Scope), strconv.FormatBool(svc.Disabled),
		svc.UpdatedAt.Format(time.RFC3339), svc.CreatedBy, svc.Name,
	).Int()
	if err != nil {
		return err
	}
	switch res {
	case -1:
		return ErrLimit
	case 0:
		return ErrExists
	default:
		return nil
	}
}

// GetService возвращает сервисный аккаунт или ErrNotFound.
func (s *Store) GetService(ctx context.Context, name string) (*ServiceAccount, error) {
	m, err := s.rdb.HGetAll(ctx, serviceKey(name)).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, ErrNotFound
	}
	return decodeService(name, m), nil
}

func decodeService(name string, m map[string]string) *ServiceAccount {
	svc := &ServiceAccount{
		Name: name, Description: m["description"], Scope: ServiceScope(m["scope"]),
		CreatedBy: m["created_by"],
	}
	svc.Disabled, _ = strconv.ParseBool(m["disabled"])
	svc.CreatedAt, _ = time.Parse(time.RFC3339, m["created_at"])
	svc.UpdatedAt, _ = time.Parse(time.RFC3339, m["updated_at"])
	return svc
}

// ListServices возвращает сервисные аккаунты, удаляя осиротевшие имена.
func (s *Store) ListServices(ctx context.Context) ([]ServiceAccount, error) {
	names, err := s.rdb.SMembers(ctx, servicesIndex()).Result()
	if err != nil {
		return nil, err
	}
	out := make([]ServiceAccount, 0, len(names))
	for _, name := range names {
		svc, err := s.GetService(ctx, name)
		if errors.Is(err, ErrNotFound) {
			_ = s.rdb.SRem(ctx, servicesIndex(), name).Err()
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, *svc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// UpdateService меняет описание, права и признак отключения.
func (s *Store) UpdateService(ctx context.Context, name string, upd ServiceUpdate) (*ServiceAccount, error) {
	svc, err := s.GetService(ctx, name)
	if err != nil {
		return nil, err
	}
	if upd.Description != nil {
		svc.Description = *upd.Description
	}
	if upd.Scope != nil {
		svc.Scope = *upd.Scope
	}
	if upd.Disabled != nil {
		svc.Disabled = *upd.Disabled
	}
	svc.UpdatedAt = time.Now().UTC()
	if _, err := s.rdb.HSet(ctx, serviceKey(name), map[string]any{
		"description": svc.Description,
		"scope":       string(svc.Scope),
		"disabled":    strconv.FormatBool(svc.Disabled),
		"updated_at":  svc.UpdatedAt.Format(time.RFC3339),
	}).Result(); err != nil {
		return nil, err
	}
	return svc, nil
}

// DeleteService удаляет сервисный аккаунт и все его токены.
func (s *Store) DeleteService(ctx context.Context, name string) error {
	if _, err := s.GetService(ctx, name); err != nil {
		return err
	}
	ids, err := s.rdb.SMembers(ctx, serviceTokensKey(name)).Result()
	if err != nil {
		return err
	}
	pipe := s.rdb.TxPipeline()
	for _, id := range ids {
		pipe.Del(ctx, tokenKey(id))
	}
	pipe.Del(ctx, serviceKey(name), serviceTokensKey(name))
	pipe.SRem(ctx, servicesIndex(), name)
	_, err = pipe.Exec(ctx)
	return err
}
