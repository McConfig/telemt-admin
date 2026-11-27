package configx

const (
	APIVersion = "tgadmin.telemt/v1"
	Kind       = "InstallationBundle"

	SecretsNone     = "none"
	SecretsIncluded = "included"
)

// Block — идентификатор блока выгрузки.
type Block string

const (
	BlockSettings  Block = "settings"
	BlockTemplates Block = "limitTemplates"
	BlockProxy     Block = "proxy"
	BlockUsers     Block = "users"
	BlockServices  Block = "services"
	BlockClients   Block = "clients"
)

// Bundle — корневой документ обмена конфигурацией.
type Bundle struct {
	APIVersion string   `yaml:"apiVersion" json:"apiVersion"`
	Kind       string   `yaml:"kind" json:"kind"`
	Metadata   Metadata `yaml:"metadata" json:"metadata"`
	Spec       Spec     `yaml:"spec" json:"spec"`
}

// Metadata — сведения об источнике выгрузки.
type Metadata struct {
	Name          string `yaml:"name" json:"name"`
	CreatedAt     string `yaml:"createdAt" json:"createdAt"`
	AppVersion    string `yaml:"appVersion,omitempty" json:"appVersion,omitempty"`
	Generator     string `yaml:"generator,omitempty" json:"generator,omitempty"`
	Secrets       string `yaml:"secrets" json:"secrets"`
	SourceBaseURL string `yaml:"sourceBaseUrl,omitempty" json:"sourceBaseUrl,omitempty"`
}

// Spec — выбранные блоки.
type Spec struct {
	Settings       map[string]map[string]any `yaml:"settings,omitempty" json:"settings,omitempty"`
	LimitTemplates []LimitTemplate           `yaml:"limitTemplates,omitempty" json:"limitTemplates,omitempty"`
	Proxy          *Proxy                    `yaml:"proxy,omitempty" json:"proxy,omitempty"`
	Users          []User                    `yaml:"users,omitempty" json:"users,omitempty"`
	Services       []Service                 `yaml:"services,omitempty" json:"services,omitempty"`
	Clients        []Client                  `yaml:"clients,omitempty" json:"clients,omitempty"`
}

// Proxy — конфигурация telemt.
type Proxy struct {
	Revision string         `yaml:"revision,omitempty" json:"revision,omitempty"`
	Config   map[string]any `yaml:"config" json:"config"`
}

// LimitTemplate — шаблон лимитов клиента.
type LimitTemplate struct {
	Name             string   `yaml:"name" json:"name"`
	MaxIps           *uint64  `yaml:"maxIps,omitempty" json:"maxIps,omitempty"`
	MaxTcpConns      *uint64  `yaml:"maxTcpConns,omitempty" json:"maxTcpConns,omitempty"`
	DataQuotaBytes   *uint64  `yaml:"dataQuotaBytes,omitempty" json:"dataQuotaBytes,omitempty"`
	QuotaPeriod      *string  `yaml:"quotaPeriod,omitempty" json:"quotaPeriod,omitempty"`
	RateLimitUpBps   *uint64  `yaml:"rateLimitUpBps,omitempty" json:"rateLimitUpBps,omitempty"`
	RateLimitDownBps *uint64  `yaml:"rateLimitDownBps,omitempty" json:"rateLimitDownBps,omitempty"`
	ExpirationDays   *int     `yaml:"expirationDays,omitempty" json:"expirationDays,omitempty"`
	SourceDeny       []string `yaml:"sourceDeny,omitempty" json:"sourceDeny,omitempty"`
	RiskWarnIps      *int     `yaml:"riskWarnIps,omitempty" json:"riskWarnIps,omitempty"`
	RiskHighIps      *int     `yaml:"riskHighIps,omitempty" json:"riskHighIps,omitempty"`
	RiskDistinct7d   *int     `yaml:"riskDistinct7d,omitempty" json:"riskDistinct7d,omitempty"`
	RiskNewCountries *bool    `yaml:"riskNewCountries,omitempty" json:"riskNewCountries,omitempty"`
	Default          bool     `yaml:"default,omitempty" json:"default,omitempty"`
}

// User — пользователь интерфейса.
type User struct {
	Name         string `yaml:"name" json:"name"`
	Role         string `yaml:"role" json:"role"`
	Disabled     bool   `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	TOTPEnabled  bool   `yaml:"totpEnabled,omitempty" json:"totpEnabled,omitempty"`
	PasswordHash string `yaml:"passwordHash,omitempty" json:"passwordHash,omitempty"`
}

// Service — сервисный аккаунт.
type Service struct {
	Name        string  `yaml:"name" json:"name"`
	Description string  `yaml:"description,omitempty" json:"description,omitempty"`
	Scope       string  `yaml:"scope" json:"scope"`
	Disabled    bool    `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	Tokens      []Token `yaml:"tokens,omitempty" json:"tokens,omitempty"`
}

// Token — хеш сервисного токена.
type Token struct {
	ID         string  `yaml:"id" json:"id"`
	Label      string  `yaml:"label,omitempty" json:"label,omitempty"`
	CreatedBy  string  `yaml:"createdBy,omitempty" json:"createdBy,omitempty"`
	CreatedAt  string  `yaml:"createdAt,omitempty" json:"createdAt,omitempty"`
	ExpiresAt  *string `yaml:"expiresAt,omitempty" json:"expiresAt,omitempty"`
	SecretHash string  `yaml:"secretHash" json:"secretHash"`
}

// Client — клиент telemt и его настройки в tgadmin.
type Client struct {
	Username          string   `yaml:"username" json:"username"`
	Enabled           bool     `yaml:"enabled" json:"enabled"`
	MaxTcpConns       *uint64  `yaml:"maxTcpConns,omitempty" json:"maxTcpConns,omitempty"`
	MaxUniqueIps      *uint64  `yaml:"maxUniqueIps,omitempty" json:"maxUniqueIps,omitempty"`
	ExpirationRFC3339 *string  `yaml:"expirationRFC3339,omitempty" json:"expirationRFC3339,omitempty"`
	DataQuotaBytes    *uint64  `yaml:"dataQuotaBytes,omitempty" json:"dataQuotaBytes,omitempty"`
	QuotaPeriod       *string  `yaml:"quotaPeriod,omitempty" json:"quotaPeriod,omitempty"`
	RateLimitUpBps    *uint64  `yaml:"rateLimitUpBps,omitempty" json:"rateLimitUpBps,omitempty"`
	RateLimitDownBps  *uint64  `yaml:"rateLimitDownBps,omitempty" json:"rateLimitDownBps,omitempty"`
	SourceDeny        []string `yaml:"sourceDeny,omitempty" json:"sourceDeny,omitempty"`
	Note              string   `yaml:"note,omitempty" json:"note,omitempty"`
	Trusted           *Trusted `yaml:"trusted,omitempty" json:"trusted,omitempty"`
	Risk              *Risk    `yaml:"risk,omitempty" json:"risk,omitempty"`
	Secret            string   `yaml:"secret,omitempty" json:"secret,omitempty"`
}

// Trusted — доверенные страны и адреса клиента.
type Trusted struct {
	Countries []string `yaml:"countries,omitempty" json:"countries,omitempty"`
	IPs       []string `yaml:"ips,omitempty" json:"ips,omitempty"`
}

// Risk — переопределения риск-порогов клиента.
type Risk struct {
	WarnIps      *int  `yaml:"warnIps,omitempty" json:"warnIps,omitempty"`
	HighIps      *int  `yaml:"highIps,omitempty" json:"highIps,omitempty"`
	Distinct7d   *int  `yaml:"distinct7d,omitempty" json:"distinct7d,omitempty"`
	NewCountries *bool `yaml:"newCountries,omitempty" json:"newCountries,omitempty"`
}

// HasBlock сообщает, присутствует ли блок в файле.
func (b *Bundle) HasBlock(id Block) bool {
	switch id {
	case BlockSettings:
		return b.Spec.Settings != nil
	case BlockTemplates:
		return b.Spec.LimitTemplates != nil
	case BlockProxy:
		return b.Spec.Proxy != nil
	case BlockUsers:
		return b.Spec.Users != nil
	case BlockServices:
		return b.Spec.Services != nil
	case BlockClients:
		return b.Spec.Clients != nil
	}
	return false
}
