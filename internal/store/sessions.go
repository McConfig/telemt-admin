package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"sort"
	"time"
)

// Session — сессия пользователя UI.
type Session struct {
	ID        string
	User      string
	IP        string
	UserAgent string
	CSRF      string
	CreatedAt time.Time
}

func sessionKey(id string) string { return "auth:session:" + id }
func sessionsKey(u string) string { return "auth:sessions:" + u }

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func sessionID(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateSession создаёт сессию и возвращает токен для cookie.
func (s *Store) CreateSession(ctx context.Context, user, ip, ua string, ttl time.Duration) (string, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", err
	}
	csrf, err := randomToken(32)
	if err != nil {
		return "", err
	}
	id := sessionID(token)
	key := sessionKey(id)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, key, map[string]any{
		"user":       user,
		"ip":         ip,
		"user_agent": ua,
		"csrf":       csrf,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	})
	pipe.Expire(ctx, key, ttl)
	pipe.SAdd(ctx, sessionsKey(user), id)
	pipe.Expire(ctx, sessionsKey(user), ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", err
	}
	return token, nil
}

// GetSession возвращает сессию по токену или ErrNotFound.
func (s *Store) GetSession(ctx context.Context, token string) (*Session, error) {
	if token == "" {
		return nil, ErrNotFound
	}
	id := sessionID(token)
	m, err := s.rdb.HGetAll(ctx, sessionKey(id)).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, ErrNotFound
	}
	sess := &Session{
		ID: id, User: m["user"], IP: m["ip"],
		UserAgent: m["user_agent"], CSRF: m["csrf"],
	}
	sess.CreatedAt, _ = time.Parse(time.RFC3339, m["created_at"])
	return sess, nil
}

// DeleteSession удаляет одну сессию по токену.
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	id := sessionID(token)
	sess, err := s.GetSession(ctx, token)
	if err == nil {
		_ = s.rdb.SRem(ctx, sessionsKey(sess.User), id).Err()
	}
	return s.rdb.Del(ctx, sessionKey(id)).Err()
}

// DeleteSessionByID удаляет сессию по идентификатору.
func (s *Store) DeleteSessionByID(ctx context.Context, user, id string) error {
	if err := s.rdb.Del(ctx, sessionKey(id)).Err(); err != nil {
		return err
	}
	return s.rdb.SRem(ctx, sessionsKey(user), id).Err()
}

// DeleteUserSessions удаляет все сессии пользователя, кроме токена exceptToken.
func (s *Store) DeleteUserSessions(ctx context.Context, user, exceptToken string) error {
	ids, err := s.rdb.SMembers(ctx, sessionsKey(user)).Result()
	if err != nil {
		return err
	}
	keep := ""
	if exceptToken != "" {
		keep = sessionID(exceptToken)
	}
	for _, id := range ids {
		if id == keep {
			continue
		}
		if err := s.rdb.Del(ctx, sessionKey(id)).Err(); err != nil {
			return err
		}
		_ = s.rdb.SRem(ctx, sessionsKey(user), id).Err()
	}
	return nil
}

// ListSessions возвращает активные сессии пользователя, новые сверху.
func (s *Store) ListSessions(ctx context.Context, user string) ([]Session, error) {
	ids, err := s.rdb.SMembers(ctx, sessionsKey(user)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]Session, 0, len(ids))
	for _, id := range ids {
		m, err := s.rdb.HGetAll(ctx, sessionKey(id)).Result()
		if err != nil {
			return nil, err
		}
		if len(m) == 0 {
			_ = s.rdb.SRem(ctx, sessionsKey(user), id).Err()
			continue
		}
		sess := Session{ID: id, User: m["user"], IP: m["ip"], UserAgent: m["user_agent"], CSRF: m["csrf"]}
		sess.CreatedAt, _ = time.Parse(time.RFC3339, m["created_at"])
		out = append(out, sess)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}