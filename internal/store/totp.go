package store

import (
	"context"
	"encoding/json"
	"strconv"
	"time"
)

// SetTOTPPending сохраняет зашифрованный секрет до подтверждения кодом.
func (s *Store) SetTOTPPending(ctx context.Context, name, encrypted string) error {
	return s.rdb.HSet(ctx, userKey(name), map[string]any{
		"totp_pending": encrypted,
	}).Err()
}

// EnableTOTP включает второй фактор и сохраняет резервные коды.
func (s *Store) EnableTOTP(ctx context.Context, name, encrypted string, counter int64, recovery []string) error {
	raw, err := json.Marshal(recovery)
	if err != nil {
		return err
	}
	return s.rdb.HSet(ctx, userKey(name), map[string]any{
		"totp_enabled":      "true",
		"totp_secret":       encrypted,
		"totp_pending":      "",
		"totp_last_counter": strconv.FormatInt(counter, 10),
		"totp_created_at":   time.Now().UTC().Format(time.RFC3339),
		"totp_recovery":     string(raw),
	}).Err()
}

// DisableTOTP выключает второй фактор и стирает секреты.
func (s *Store) DisableTOTP(ctx context.Context, name string) error {
	return s.rdb.HSet(ctx, userKey(name), map[string]any{
		"totp_enabled":      "false",
		"totp_secret":       "",
		"totp_pending":      "",
		"totp_last_counter": "0",
		"totp_created_at":   "",
		"totp_recovery":     "",
	}).Err()
}

// SetTOTPLastCounter запоминает использованный шаг, чтобы код нельзя было
// повторить.
func (s *Store) SetTOTPLastCounter(ctx context.Context, name string, counter int64) error {
	return s.rdb.HSet(ctx, userKey(name), "totp_last_counter", strconv.FormatInt(counter, 10)).Err()
}

// SetRecoveryCodes заменяет список хешей резервных кодов.
func (s *Store) SetRecoveryCodes(ctx context.Context, name string, hashes []string) error {
	raw, err := json.Marshal(hashes)
	if err != nil {
		return err
	}
	return s.rdb.HSet(ctx, userKey(name), "totp_recovery", string(raw)).Err()
}

// ConsumeRecoveryCode атомарно удаляет использованный резервный код.
func (s *Store) ConsumeRecoveryCode(ctx context.Context, name, hash string) (bool, error) {
	const script = `
local raw = redis.call('HGET', KEYS[1], 'totp_recovery')
if not raw or raw == '' then return 0 end
local list = cjson.decode(raw)
for i, item in ipairs(list) do
  if item == ARGV[1] then
    table.remove(list, i)
    redis.call('HSET', KEYS[1], 'totp_recovery', cjson.encode(list))
    return 1
  end
end
return 0
`
	res, err := s.rdb.Eval(ctx, script, []string{userKey(name)}, hash).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// --- ожидающая второй фактор сессия ---

func pendingTOTPKey(id string) string { return "auth:totp:" + id }

// PendingTOTP — короткоживущее состояние между паролем и кодом.
type PendingTOTP struct {
	User      string
	IP        string
	UserAgent string
	Attempts  int64
}

// CreatePendingTOTP создаёт временный токен на время ввода кода.
func (s *Store) CreatePendingTOTP(ctx context.Context, user, ip, ua string, ttl time.Duration) (string, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", err
	}
	key := pendingTOTPKey(sessionID(token))
	if err := s.rdb.HSet(ctx, key, map[string]any{
		"user": user, "ip": ip, "user_agent": ua, "attempts": 0,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	}).Err(); err != nil {
		return "", err
	}
	if err := s.rdb.Expire(ctx, key, ttl).Err(); err != nil {
		return "", err
	}
	return token, nil
}

// GetPendingTOTP возвращает состояние по токену или ErrNotFound.
func (s *Store) GetPendingTOTP(ctx context.Context, token string) (*PendingTOTP, error) {
	if token == "" {
		return nil, ErrNotFound
	}
	key := pendingTOTPKey(sessionID(token))
	m, err := s.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, ErrNotFound
	}
	attempts, _ := strconv.ParseInt(m["attempts"], 10, 64)
	return &PendingTOTP{User: m["user"], IP: m["ip"], UserAgent: m["user_agent"], Attempts: attempts}, nil
}

// IncrPendingTOTPAttempts увеличивает счётчик попыток и возвращает его.
func (s *Store) IncrPendingTOTPAttempts(ctx context.Context, token string) (int64, error) {
	key := pendingTOTPKey(sessionID(token))
	n, err := s.rdb.HIncrBy(ctx, key, "attempts", 1).Result()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// DeletePendingTOTP удаляет временный токен.
func (s *Store) DeletePendingTOTP(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.rdb.Del(ctx, pendingTOTPKey(sessionID(token))).Err()
}
