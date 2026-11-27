package store

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/redis/go-redis/v9"
)

// MaxPasskeys — максимум passkeys на пользователя.
const MaxPasskeys = 10

// Passkey — публичный credential record WebAuthn.
type Passkey struct {
	ID             string
	User           string
	Name           string
	Credential     webauthn.Credential
	CreatedAt      time.Time
	LastUsedAt     time.Time
	BackupEligible bool
	BackupState    bool
	SignCount      uint32
}

func passkeySetKey(user string) string { return "auth:webauthn:" + user }
func passkeyKey(id string) string      { return "auth:webauthn_cred:" + id }

var createPasskeyScript = redis.NewScript(`
local count = redis.call('SCARD', KEYS[1])
if count >= tonumber(ARGV[1]) then return -1 end
if redis.call('HSETNX', KEYS[2], 'user', ARGV[2]) == 0 then return 0 end
redis.call('HSET', KEYS[2],
  'name', ARGV[3], 'credential', ARGV[4], 'created_at', ARGV[5],
  'sign_count', ARGV[6], 'backup_eligible', ARGV[7], 'backup_state', ARGV[8])
redis.call('SADD', KEYS[1], ARGV[9])
return 1
`)

// CreatePasskey сохраняет credential record и проверяет лимит.
func (s *Store) CreatePasskey(ctx context.Context, p Passkey) error {
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	raw, err := json.Marshal(p.Credential)
	if err != nil {
		return err
	}
	res, err := createPasskeyScript.Run(ctx, s.rdb,
		[]string{passkeySetKey(p.User), passkeyKey(p.ID)},
		MaxPasskeys, p.User, p.Name, raw, p.CreatedAt.Format(time.RFC3339),
		p.SignCount, strconv.FormatBool(p.BackupEligible), strconv.FormatBool(p.BackupState), p.ID,
	).Int()
	if err != nil {
		return err
	}
	if res < 0 {
		return ErrLimit
	}
	if res == 0 {
		return ErrExists
	}
	return nil
}

// GetPasskey возвращает запись по публичному ID.
func (s *Store) GetPasskey(ctx context.Context, id string) (*Passkey, error) {
	m, err := s.rdb.HGetAll(ctx, passkeyKey(id)).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, ErrNotFound
	}
	return decodePasskey(id, m)
}

// ListPasskeys возвращает passkeys пользователя.
func (s *Store) ListPasskeys(ctx context.Context, user string) ([]Passkey, error) {
	ids, err := s.rdb.SMembers(ctx, passkeySetKey(user)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]Passkey, 0, len(ids))
	for _, id := range ids {
		p, err := s.GetPasskey(ctx, id)
		if errors.Is(err, ErrNotFound) {
			_ = s.rdb.SRem(ctx, passkeySetKey(user), id).Err()
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, nil
}

// CountPasskeys возвращает число passkeys пользователя.
func (s *Store) CountPasskeys(ctx context.Context, user string) (int64, error) {
	return s.rdb.SCard(ctx, passkeySetKey(user)).Result()
}

// DeletePasskey удаляет passkey пользователя.
func (s *Store) DeletePasskey(ctx context.Context, user, id string) error {
	p, err := s.GetPasskey(ctx, id)
	if err != nil {
		return err
	}
	if p.User != user {
		return ErrNotFound
	}
	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, passkeyKey(id))
	pipe.SRem(ctx, passkeySetKey(user), id)
	_, err = pipe.Exec(ctx)
	return err
}

// DeleteAllPasskeys удаляет все passkeys пользователя.
func (s *Store) DeleteAllPasskeys(ctx context.Context, user string) error {
	ids, err := s.rdb.SMembers(ctx, passkeySetKey(user)).Result()
	if err != nil {
		return err
	}
	pipe := s.rdb.TxPipeline()
	for _, id := range ids {
		pipe.Del(ctx, passkeyKey(id))
	}
	pipe.Del(ctx, passkeySetKey(user))
	_, err = pipe.Exec(ctx)
	return err
}

// UpdatePasskeyUsage обновляет sign count и время последнего входа.
func (s *Store) UpdatePasskeyUsage(ctx context.Context, id string, signCount uint32, at time.Time) error {
	p, err := s.GetPasskey(ctx, id)
	if err != nil {
		return err
	}
	p.Credential.Authenticator.SignCount = signCount
	p.SignCount = signCount
	raw, err := json.Marshal(p.Credential)
	if err != nil {
		return err
	}
	return s.rdb.HSet(ctx, passkeyKey(id), map[string]any{
		"credential": raw, "sign_count": signCount,
		"last_used_at":    at.UTC().Format(time.RFC3339),
		"backup_eligible": strconv.FormatBool(p.BackupEligible),
		"backup_state":    strconv.FormatBool(p.BackupState),
	}).Err()
}

func decodePasskey(id string, m map[string]string) (*Passkey, error) {
	p := &Passkey{ID: id, User: m["user"], Name: m["name"]}
	if err := json.Unmarshal([]byte(m["credential"]), &p.Credential); err != nil {
		return nil, err
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, m["created_at"])
	p.LastUsedAt, _ = time.Parse(time.RFC3339, m["last_used_at"])
	if n, err := strconv.ParseUint(m["sign_count"], 10, 32); err == nil {
		p.SignCount = uint32(n)
	}
	p.BackupEligible, _ = strconv.ParseBool(m["backup_eligible"])
	p.BackupState, _ = strconv.ParseBool(m["backup_state"])
	return p, nil
}
