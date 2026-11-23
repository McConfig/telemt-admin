package telemt

import "context"

// Links — ссылки пользователя, которые вернул telemt.
type Links struct {
	Classic []string `json:"classic"`
	Secure  []string `json:"secure"`
	TLS     []string `json:"tls"`
}

// User — пользователь прокси.
type User struct {
	Username           string   `json:"username"`
	Enabled            bool     `json:"enabled"`
	InRuntime          bool     `json:"in_runtime"`
	MaxTCPConns        *uint64  `json:"max_tcp_conns"`
	MaxUniqueIPs       *uint64  `json:"max_unique_ips"`
	ExpirationRFC3339  *string  `json:"expiration_rfc3339"`
	DataQuotaBytes     *uint64  `json:"data_quota_bytes"`
	QuotaPeriod        *string  `json:"quota_period"`
	RateLimitUpBps     *uint64  `json:"rate_limit_up_bps"`
	RateLimitDownBps   *uint64  `json:"rate_limit_down_bps"`
	CurrentConnections uint64   `json:"current_connections"`
	ActiveUniqueIPs    uint64   `json:"active_unique_ips"`
	ActiveUniqueIPList []string `json:"active_unique_ips_list"`
	RecentUniqueIPList []string `json:"recent_unique_ips_list"`
	TotalOctets        uint64   `json:"total_octets"`
	Links              *Links   `json:"links"`
}

// Disabled сообщает, заблокирован ли пользователь.
func (u User) Disabled() bool { return !u.Enabled }

// Link возвращает первую ссылку выбранного режима.
func (u User) Link(mode string) string {
	if u.Links == nil {
		return ""
	}
	var list []string
	switch mode {
	case "classic":
		list = u.Links.Classic
	case "secure":
		list = u.Links.Secure
	default:
		list = u.Links.TLS
	}
	if len(list) == 0 {
		return ""
	}
	return list[0]
}

// ListUsers возвращает всех пользователей прокси.
func (c *Client) ListUsers(ctx context.Context) ([]User, error) {
	var users []User
	if err := c.do(ctx, "GET", "/v1/users", nil, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// GetUser возвращает одного пользователя.
func (c *Client) GetUser(ctx context.Context, name string) (*User, error) {
	var u User
	if err := c.do(ctx, "GET", "/v1/users/"+name, nil, &u); err != nil {
		return nil, err
	}
	return &u, nil
}
