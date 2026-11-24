package store

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

// Role — роль пользователя интерфейса.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleViewer Role = "viewer"
)

// User — учётная запись UI.
type User struct {
	Name              string
	Hash              string
	Role              Role
	CreatedAt         time.Time
	PasswordChangedAt time.Time
	Disabled          bool

	// TOTP
	TOTPEnabled     bool
	TOTPSecret      string
	TOTPPending     string
	TOTPLastCounter int64
	TOTPCreatedAt   time.Time
	RecoveryCodes   []string
}

func userKey(name string) string { return "auth:user:" + name }
func usersIndex() string         { return "auth:users" }

// CreateUser создаёт пользователя; при существующем имени возвращает ErrExists.
func (s *Store) CreateUser(ctx context.Context, u User) error {
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now().UTC()
	}
	if u.PasswordChangedAt.IsZero() {
		u.PasswordChangedAt = u.CreatedAt
	}
	key := userKey(u.Name)
	ok, err := s.rdb.HSetNX(ctx, key, "pw_hash", u.Hash).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrExists
	}
	_, err = s.rdb.HSet(ctx, key, map[string]any{
		"role":                string(u.Role),
		"created_at":          u.CreatedAt.UTC().Format(time.RFC3339),
		"password_changed_at": u.PasswordChangedAt.UTC().Format(time.RFC3339),
		"disabled":            strconv.FormatBool(u.Disabled),
	}).Result()
	if err != nil {
		return err
	}
	return s.rdb.SAdd(ctx, usersIndex(), u.Name).Err()
}

// GetUser возвращает пользователя или ErrNotFound.
func (s *Store) GetUser(ctx context.Context, name string) (*User, error) {
	m, err := s.rdb.HGetAll(ctx, userKey(name)).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, ErrNotFound
	}
	return decodeUser(name, m), nil
}

func decodeUser(name string, m map[string]string) *User {
	u := &User{Name: name, Hash: m["pw_hash"], Role: Role(m["role"])}
	u.CreatedAt, _ = time.Parse(time.RFC3339, m["created_at"])
	u.PasswordChangedAt, _ = time.Parse(time.RFC3339, m["password_changed_at"])
	u.Disabled, _ = strconv.ParseBool(m["disabled"])
	u.TOTPEnabled, _ = strconv.ParseBool(m["totp_enabled"])
	u.TOTPSecret = m["totp_secret"]
	u.TOTPPending = m["totp_pending"]
	u.TOTPLastCounter, _ = strconv.ParseInt(m["totp_last_counter"], 10, 64)
	u.TOTPCreatedAt, _ = time.Parse(time.RFC3339, m["totp_created_at"])
	if raw := m["totp_recovery"]; raw != "" {
		_ = json.Unmarshal([]byte(raw), &u.RecoveryCodes)
	}
	return u
}

// ListUsers возвращает всех пользователей UI.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	names, err := s.rdb.SMembers(ctx, usersIndex()).Result()
	if err != nil {
		return nil, err
	}
	out := make([]User, 0, len(names))
	for _, name := range names {
		u, err := s.GetUser(ctx, name)
		if errors.Is(err, ErrNotFound) {
			_ = s.rdb.SRem(ctx, usersIndex(), name).Err()
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, nil
}

// DeleteUser удаляет пользователя и его сессии.
func (s *Store) DeleteUser(ctx context.Context, name string) error {
	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, userKey(name))
	pipe.SRem(ctx, usersIndex(), name)
	pipe.Del(ctx, "auth:sessions:"+name)
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	return s.DeleteUserSessions(ctx, name, "")
}

// SetPassword меняет хэш пароля.
func (s *Store) SetPassword(ctx context.Context, name, hash string) error {
	exists, err := s.rdb.Exists(ctx, userKey(name)).Result()
	if err != nil {
		return err
	}
	if exists == 0 {
		return ErrNotFound
	}
	return s.rdb.HSet(ctx, userKey(name), map[string]any{
		"pw_hash":             hash,
		"password_changed_at": time.Now().UTC().Format(time.RFC3339),
	}).Err()
}

// SetDisabled включает или отключает учётную запись.
func (s *Store) SetDisabled(ctx context.Context, name string, disabled bool) error {
	exists, err := s.rdb.Exists(ctx, userKey(name)).Result()
	if err != nil {
		return err
	}
	if exists == 0 {
		return ErrNotFound
	}
	return s.rdb.HSet(ctx, userKey(name), "disabled", strconv.FormatBool(disabled)).Err()
}

// CountUsers возвращает число учётных записей UI.
func (s *Store) CountUsers(ctx context.Context) (int64, error) {
	return s.rdb.SCard(ctx, usersIndex()).Result()
}

// SetRole меняет роль пользователя UI.
func (s *Store) SetRole(ctx context.Context, name string, role Role) error {
	exists, err := s.rdb.Exists(ctx, userKey(name)).Result()
	if err != nil {
		return err
	}
	if exists == 0 {
		return ErrNotFound
	}
	return s.rdb.HSet(ctx, userKey(name), "role", string(role)).Err()
}
