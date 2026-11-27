package configx

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/McConfig/telemt-admin/internal/telemt"
)

// ExportOptions — выбранные блоки и режим секретов.
type ExportOptions struct {
	Blocks     []Block
	Secrets    bool
	Name       string
	AppVersion string
	BaseURL    string
}

// Export собирает бандл.
func (s *Service) Export(ctx context.Context, opts ExportOptions) (*Bundle, error) {
	secrets := SecretsNone
	if opts.Secrets {
		secrets = SecretsIncluded
	}
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{
			Name: opts.Name, CreatedAt: s.now().UTC().Format(time.RFC3339),
			AppVersion: opts.AppVersion, Generator: "tgadmin",
			Secrets: secrets, SourceBaseURL: opts.BaseURL,
		},
	}
	for _, block := range opts.Blocks {
		switch block {
		case BlockSettings:
			b.Spec.Settings = s.exportSettings(ctx, opts.Secrets)
		case BlockTemplates:
			templates, err := s.exportTemplates(ctx)
			if err != nil {
				return nil, err
			}
			b.Spec.LimitTemplates = templates
		case BlockProxy:
			if s.telemt == nil {
				return nil, fmt.Errorf("proxy_unavailable: TELEMT_API_URL не задан")
			}
			cfg, err := s.telemt.GetConfig(ctx)
			if err != nil {
				return nil, err
			}
			b.Spec.Proxy = &Proxy{Revision: cfg.Revision, Config: cfg.Config}
		case BlockUsers:
			users, err := s.exportUsers(ctx, opts.Secrets)
			if err != nil {
				return nil, err
			}
			b.Spec.Users = users
		case BlockServices:
			services, err := s.exportServices(ctx, opts.Secrets)
			if err != nil {
				return nil, err
			}
			b.Spec.Services = services
		case BlockClients:
			clients, err := s.exportClients(ctx, opts.Secrets)
			if err != nil {
				return nil, err
			}
			b.Spec.Clients = clients
		default:
			return nil, fmt.Errorf("bad_request: неизвестный блок %q", block)
		}
	}
	return b, nil
}

var sensitiveSettings = map[string]bool{
	"notifications.webhook_url": true,
}

func (s *Service) exportSettings(ctx context.Context, includeSecrets bool) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, f := range s.settings.ExportValues(ctx) {
		if f.Section == "infrastructure" {
			continue
		}
		if (f.Secret || sensitiveSettings[f.Key]) && !includeSecrets {
			continue
		}
		section := out[f.Section]
		if section == nil {
			section = map[string]any{}
			out[f.Section] = section
		}
		key := strings.TrimPrefix(f.Key, f.Section+".")
		section[key] = f.Value
	}
	if len(out) == 0 {
		return map[string]map[string]any{}
	}
	return out
}

func (s *Service) exportTemplates(ctx context.Context) ([]LimitTemplate, error) {
	list, err := s.st.ListTemplates(ctx)
	if err != nil {
		return nil, err
	}
	def, _ := s.st.DefaultTemplate(ctx)
	out := make([]LimitTemplate, 0, len(list))
	for _, t := range list {
		out = append(out, LimitTemplate{
			Name: t.Name, MaxIps: t.MaxIps, MaxTcpConns: t.MaxTcpConns,
			DataQuotaBytes: t.DataQuotaBytes, QuotaPeriod: t.QuotaPeriod,
			RateLimitUpBps: t.RateLimitUpBps, RateLimitDownBps: t.RateLimitDownBps,
			ExpirationDays: t.ExpirationDays, SourceDeny: t.SourceDeny,
			RiskWarnIps: t.RiskWarnIps, RiskHighIps: t.RiskHighIps,
			RiskDistinct7d: t.RiskDistinct7d, RiskNewCountries: t.RiskNewCountries,
			Default: t.Name == def,
		})
	}
	return out, nil
}

func (s *Service) exportUsers(ctx context.Context, includeSecrets bool) ([]User, error) {
	list, err := s.st.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]User, 0, len(list))
	for _, u := range list {
		item := User{Name: u.Name, Role: string(u.Role), Disabled: u.Disabled, TOTPEnabled: u.TOTPEnabled}
		if includeSecrets {
			item.PasswordHash = u.Hash
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Service) exportServices(ctx context.Context, includeSecrets bool) ([]ServiceAccount, error) {
	list, err := s.st.ListServices(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ServiceAccount, 0, len(list))
	for _, svc := range list {
		item := ServiceAccount{Name: svc.Name, Description: svc.Description, Scope: string(svc.Scope), Disabled: svc.Disabled}
		if includeSecrets {
			tokens, err := s.st.ExportServiceTokens(ctx, svc.Name)
			if err != nil {
				return nil, err
			}
			for _, t := range tokens {
				item.Tokens = append(item.Tokens, Token{
					ID: t.ID, Label: t.Label, CreatedBy: t.CreatedBy,
					CreatedAt: t.CreatedAt.UTC().Format(time.RFC3339),
					ExpiresAt: formatOptionalTime(t.ExpiresAt), SecretHash: t.SecretHash,
				})
			}
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Service) exportClients(ctx context.Context, includeSecrets bool) ([]Client, error) {
	if s.telemt == nil {
		return nil, fmt.Errorf("proxy_unavailable: TELEMT_API_URL не задан")
	}
	users, err := s.telemt.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Client, 0, len(users))
	for _, u := range users {
		item := Client{
			Username: u.Username, Enabled: u.Enabled,
			MaxTcpConns: u.MaxTCPConns, MaxUniqueIps: u.MaxUniqueIPs,
			ExpirationRFC3339: u.ExpirationRFC3339, DataQuotaBytes: u.DataQuotaBytes,
			QuotaPeriod: u.QuotaPeriod, RateLimitUpBps: u.RateLimitUpBps,
			RateLimitDownBps: u.RateLimitDownBps,
		}
		item.Note, _ = s.st.Note(ctx, u.Username)
		if trusted, err := s.st.GetTrusted(ctx, u.Username); err == nil {
			if len(trusted.Countries) > 0 || len(trusted.IPs) > 0 {
				item.Trusted = &Trusted{Countries: trusted.Countries, IPs: trusted.IPs}
			}
		}
		if risk, ok, err := s.st.GetRisk(ctx, u.Username); err == nil && ok {
			item.Risk = &Risk{
				WarnIps: risk.WarnIPs, HighIps: risk.HighIPs,
				Distinct7d: risk.Distinct7d, NewCountries: risk.NewCountries,
			}
		}
		if includeSecrets && u.Links != nil {
			item.Secret = secretFromLinks(u.Links)
		}
		out = append(out, item)
	}
	return out, nil
}

func formatOptionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	v := t.UTC().Format(time.RFC3339)
	return &v
}

func secretFromLinks(l *telemt.Links) string {
	for _, group := range [][]string{l.Classic, l.Secure, l.TLS} {
		for _, link := range group {
			u, err := url.Parse(link)
			if err != nil {
				continue
			}
			raw := u.Query().Get("secret")
			if (strings.HasPrefix(raw, "ee") || strings.HasPrefix(raw, "dd")) && len(raw) >= 34 {
				return raw[2:34]
			}
			if len(raw) == 32 {
				return raw
			}
		}
	}
	return ""
}
