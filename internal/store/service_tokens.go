package store

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/redis/go-redis/v9"
)

// MaxServiceTokens — максимум активных токенов на сервис.
const MaxServiceTokens = 20

// ErrInvalidToken — неверный, истёкший или отозванный токен.
var ErrInvalidToken = errors.New("неверный или истёкший токен")

var createServiceTokenScript = redis.NewScript(`
local count = redis.call('SCARD', KEYS[1])
if count >= tonumber(ARGV[1]) then return -1 end
redis.call('HSET', KEYS[2],
  'service', ARGV[2], 'label', ARGV[4], 'secret_hash', ARGV[5],
  'created_at', ARGV[6], 'created_by', ARGV[7])
if ARGV[8] ~= '' then
  redis.call('HSET', KEYS[2], 'expires_at', ARGV[8])
  redis.call('PEXPIREAT', KEYS[2], ARGV[9])
end
redis.call('SADD', KEYS[1], ARGV[3])
return 1
`)

// ServiceToken — метаданные токена без секрета.
type ServiceToken struct {
	ID         string
	Service    string
	Label      string
	CreatedAt  time.Time
	CreatedBy  string
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
}

// ServiceAuth — результат проверки Bearer-токена.
type ServiceAuth struct {
	Service ServiceAccount
	TokenID string
}

// CreateServiceToken выпускает токен и возвращает его полное значение один раз.
func (s *Store) CreateServiceToken(
	ctx context.Context, service, label, createdBy string, expiresAt *time.Time,
) (string, ServiceToken, error) {
	if _, err := s.GetService(ctx, service); err != nil {
		return "", ServiceToken{}, err
	}
	if err := s.pruneServiceTokens(ctx, service); err != nil {
		return "", ServiceToken{}, err
	}
	count, err := s.rdb.SCard(ctx, serviceTokensKey(service)).Result()
	if err != nil {
		return "", ServiceToken{}, err
	}
	if count >= MaxServiceTokens {
		return "", ServiceToken{}, ErrLimit
	}
	id, full, err := auth.NewServiceToken()
	if err != nil {
		return "", ServiceToken{}, err
	}
	_, secret, _ := auth.ParseServiceToken(full)
	now := time.Now().UTC()
	tok := ServiceToken{
		ID: id, Service: service, Label: label, CreatedAt: now, CreatedBy: createdBy,
	}
	if expiresAt != nil {
		exp := expiresAt.UTC()
		tok.ExpiresAt = &exp
	}
	expires := ""
	ttl := ""
	if tok.ExpiresAt != nil {
		expires = tok.ExpiresAt.Format(time.RFC3339)
		ttl = strconv.FormatInt(tok.ExpiresAt.UnixMilli(), 10)
	}
	res, err := createServiceTokenScript.Run(ctx, s.rdb,
		[]string{serviceTokensKey(service), tokenKey(id)},
		MaxServiceTokens, service, id, label,
		auth.HashServiceSecret(secret), now.Format(time.RFC3339),
		createdBy, expires, ttl,
	).Int()
	if err != nil {
		return "", ServiceToken{}, err
	}
	if res < 0 {
		return "", ServiceToken{}, ErrLimit
	}
	return full, tok, nil
}

// ListServiceTokens возвращает токены сервиса без секретов.
func (s *Store) ListServiceTokens(ctx context.Context, service string) ([]ServiceToken, error) {
	if _, err := s.GetService(ctx, service); err != nil {
		return nil, err
	}
	if err := s.pruneServiceTokens(ctx, service); err != nil {
		return nil, err
	}
	ids, err := s.rdb.SMembers(ctx, serviceTokensKey(service)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]ServiceToken, 0, len(ids))
	for _, id := range ids {
		m, err := s.rdb.HGetAll(ctx, tokenKey(id)).Result()
		if err != nil {
			return nil, err
		}
		if len(m) == 0 {
			continue
		}
		out = append(out, decodeServiceToken(id, m))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// DeleteServiceToken отзывает токен сервиса.
func (s *Store) DeleteServiceToken(ctx context.Context, service, id string) error {
	m, err := s.rdb.HGetAll(ctx, tokenKey(id)).Result()
	if err != nil {
		return err
	}
	if len(m) == 0 || m["service"] != service {
		return ErrNotFound
	}
	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, tokenKey(id))
	pipe.SRem(ctx, serviceTokensKey(service), id)
	_, err = pipe.Exec(ctx)
	return err
}

// AuthenticateServiceToken проверяет Bearer-токен и обновляет last_used_at.
func (s *Store) AuthenticateServiceToken(ctx context.Context, raw string) (*ServiceAuth, error) {
	id, secret, ok := auth.ParseServiceToken(raw)
	if !ok {
		return nil, ErrInvalidToken
	}
	m, err := s.rdb.HGetAll(ctx, tokenKey(id)).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, ErrInvalidToken
	}
	svc, err := s.GetService(ctx, m["service"])
	if err != nil || svc.Disabled {
		return nil, ErrInvalidToken
	}
	inIndex, err := s.rdb.SIsMember(ctx, serviceTokensKey(svc.Name), id).Result()
	if err != nil || !inIndex {
		return nil, ErrInvalidToken
	}
	now := time.Now().UTC()
	if expires := m["expires_at"]; expires != "" {
		exp, err := time.Parse(time.RFC3339, expires)
		if err != nil || !now.Before(exp) {
			return nil, ErrInvalidToken
		}
	}
	if !auth.VerifyServiceSecret(secret, m["secret_hash"]) {
		return nil, ErrInvalidToken
	}
	if last, err := time.Parse(time.RFC3339, m["last_used_at"]); err != nil ||
		now.Sub(last) >= time.Minute {
		_ = s.rdb.HSet(ctx, tokenKey(id), "last_used_at", now.Format(time.RFC3339)).Err()
	}
	return &ServiceAuth{Service: *svc, TokenID: id}, nil
}

func (s *Store) pruneServiceTokens(ctx context.Context, service string) error {
	ids, err := s.rdb.SMembers(ctx, serviceTokensKey(service)).Result()
	if err != nil {
		return err
	}
	for _, id := range ids {
		n, err := s.rdb.Exists(ctx, tokenKey(id)).Result()
		if err != nil {
			return err
		}
		if n == 0 {
			_ = s.rdb.SRem(ctx, serviceTokensKey(service), id).Err()
		}
	}
	return nil
}

func decodeServiceToken(id string, m map[string]string) ServiceToken {
	tok := ServiceToken{
		ID: id, Service: m["service"], Label: m["label"], CreatedBy: m["created_by"],
	}
	tok.CreatedAt, _ = time.Parse(time.RFC3339, m["created_at"])
	if raw := m["expires_at"]; raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			tok.ExpiresAt = &t
		}
	}
	if raw := m["last_used_at"]; raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			tok.LastUsedAt = &t
		}
	}
	return tok
}
