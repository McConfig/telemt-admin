package telemt

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// CreateInput — тело запроса создания пользователя.
type CreateInput struct {
	Username          string  `json:"username"`
	Secret            string  `json:"secret,omitempty"`
	MaxTCPConns       *uint64 `json:"max_tcp_conns,omitempty"`
	MaxUniqueIPs      *uint64 `json:"max_unique_ips,omitempty"`
	ExpirationRFC3339 *string `json:"expiration_rfc3339,omitempty"`
	DataQuotaBytes    *uint64 `json:"data_quota_bytes,omitempty"`
	QuotaPeriod       *string `json:"quota_period,omitempty"`
	RateLimitUpBps    *uint64 `json:"rate_limit_up_bps,omitempty"`
	RateLimitDownBps  *uint64 `json:"rate_limit_down_bps,omitempty"`
	Enabled           *bool   `json:"enabled,omitempty"`
}

// CreateUser создаёт пользователя.
func (c *Client) CreateUser(ctx context.Context, in CreateInput) (*User, error) {
	var resp struct {
		User *User `json:"user"`
	}
	if err := c.do(ctx, "POST", "/v1/users", in, &resp); err != nil {
		return nil, err
	}
	if resp.User == nil {
		return c.GetUser(ctx, in.Username)
	}
	return resp.User, nil
}

// PatchUser меняет поля пользователя по семантике merge patch.
func (c *Client) PatchUser(ctx context.Context, name string, patch map[string]any) error {
	return c.do(ctx, "PATCH", "/v1/users/"+name, patch, nil)
}

// DeleteUser удаляет пользователя.
func (c *Client) DeleteUser(ctx context.Context, name string) error {
	return c.do(ctx, "DELETE", "/v1/users/"+name, nil, nil)
}

// RotateSecret перевыпускает секрет.
func (c *Client) RotateSecret(ctx context.Context, name string) error {
	return c.do(ctx, "POST", "/v1/users/"+name+"/rotate-secret", nil, nil)
}

// EnableUser разблокирует пользователя.
func (c *Client) EnableUser(ctx context.Context, name string) error {
	return c.do(ctx, "POST", "/v1/users/"+name+"/enable", nil, nil)
}

// DisableUser блокирует пользователя и закрывает сессии.
func (c *Client) DisableUser(ctx context.Context, name string) error {
	return c.do(ctx, "POST", "/v1/users/"+name+"/disable", nil, nil)
}

// KeyFromLink извлекает 32-hex ключ из FakeTLS-ссылки.
func KeyFromLink(link string) (string, error) {
	u, err := url.Parse(link)
	if err != nil {
		return "", fmt.Errorf("ссылка: %w", err)
	}
	secret := u.Query().Get("secret")
	if !strings.HasPrefix(secret, "ee") {
		return "", errors.New("ссылка не в формате FakeTLS")
	}
	rest := secret[2:]
	if len(rest) < 32 {
		return "", errors.New("секрет короче 32 hex")
	}
	key := rest[:32]
	if _, err := hex.DecodeString(key); err != nil {
		return "", fmt.Errorf("секрет не hex: %w", err)
	}
	return key, nil
}
