package configx

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/telemt"
)

var (
	ErrStalePlan    = errors.New("stale_plan")
	ErrPlanUsed     = errors.New("plan_used")
	ErrPlanNotFound = errors.New("plan_not_found")
)

// ApplyOptions — выбор блоков, режимов и конфликтов.
type ApplyOptions struct {
	Blocks        []Block
	Modes         map[Block]string
	Conflicts     map[string]map[string]string
	DeleteMissing bool
	ConfirmName   string
}

// BlockResult — результат применения блока.
type BlockResult struct {
	Block     Block               `json:"block"`
	Status    string              `json:"status"`
	Applied   int                 `json:"applied,omitempty"`
	Created   int                 `json:"created,omitempty"`
	Updated   int                 `json:"updated,omitempty"`
	Skipped   int                 `json:"skipped,omitempty"`
	Deleted   int                 `json:"deleted,omitempty"`
	Error     string              `json:"error,omitempty"`
	Passwords map[string]string   `json:"passwords,omitempty"`
	Links     map[string][]string `json:"links,omitempty"`
}

// Apply выполняет план импорта.
func (s *Service) Apply(ctx context.Context, plan *Plan, opts ApplyOptions) ([]BlockResult, error) {
	if plan == nil || plan.Bundle == nil {
		return nil, ErrPlanNotFound
	}
	if len(opts.Blocks) == 0 {
		return nil, errors.New("bad_request: не выбран ни один блок")
	}
	if opts.DeleteMissing && opts.ConfirmName != plan.Bundle.Metadata.Name {
		return nil, errors.New("bad_request: подтвердите удаление именем установки")
	}
	if cur := s.settings.Version(ctx); cur != plan.TargetSettingsVersion && plan.HasBlockRequested(BlockSettings, opts.Blocks) {
		return nil, ErrStalePlan
	}
	results := make([]BlockResult, 0, len(opts.Blocks))
	for _, block := range orderedBlocks(opts.Blocks) {
		if !plan.Bundle.HasBlock(block) {
			results = append(results, BlockResult{Block: block, Status: "skipped", Error: "блок отсутствует в файле"})
			continue
		}
		var res BlockResult
		var err error
		switch block {
		case BlockSettings:
			res, err = s.applySettings(ctx, plan, modeOf(opts, block))
		case BlockTemplates:
			res, err = s.applyTemplates(ctx, plan, opts.DeleteMissing)
		case BlockUsers:
			res, err = s.applyUsers(ctx, plan, opts)
		case BlockServices:
			res, err = s.applyServices(ctx, plan, opts)
		case BlockClients:
			res, err = s.applyClients(ctx, plan, opts)
		case BlockProxy:
			res, err = s.applyProxy(ctx, plan)
		}
		if err != nil {
			res.Block = block
			res.Status = "failed"
			res.Error = err.Error()
			results = append(results, res)
			return results, err
		}
		results = append(results, res)
	}
	return results, nil
}

func (p *Plan) HasBlockRequested(block Block, blocks []Block) bool {
	for _, b := range blocks {
		if b == block {
			return p.Bundle.HasBlock(block)
		}
	}
	return false
}

func orderedBlocks(blocks []Block) []Block {
	order := []Block{BlockSettings, BlockTemplates, BlockUsers, BlockServices, BlockClients, BlockProxy}
	want := map[Block]bool{}
	for _, b := range blocks {
		want[b] = true
	}
	out := make([]Block, 0, len(blocks))
	for _, b := range order {
		if want[b] {
			out = append(out, b)
		}
	}
	return out
}

func modeOf(opts ApplyOptions, block Block) string {
	if opts.Modes != nil && opts.Modes[block] == "replace" {
		return "replace"
	}
	return "merge"
}

func (s *Service) applySettings(ctx context.Context, plan *Plan, mode string) (BlockResult, error) {
	values := map[string]json.RawMessage{}
	reset := []string{}
	for section, fields := range plan.Bundle.Spec.Settings {
		present := map[string]bool{}
		for key, value := range fields {
			full := key
			if !strings.HasPrefix(key, section+".") {
				full = section + "." + key
			}
			raw, err := json.Marshal(value)
			if err != nil {
				return BlockResult{}, err
			}
			values[full] = raw
			present[full] = true
		}
		if mode == "replace" {
			for _, key := range s.settings.ActiveKeys(section) {
				if !present[key] {
					reset = append(reset, key)
				}
			}
		}
	}
	if _, err := s.settings.Validate(ctx, values, reset); err != nil {
		return BlockResult{}, err
	}
	if _, _, err := s.settings.Save(ctx, plan.TargetSettingsVersion, values, reset); err != nil {
		return BlockResult{}, err
	}
	return BlockResult{Block: BlockSettings, Status: "applied", Applied: len(values) + len(reset)}, nil
}

func (s *Service) applyTemplates(ctx context.Context, plan *Plan, deleteMissing bool) (BlockResult, error) {
	res := BlockResult{Block: BlockTemplates, Status: "applied"}
	for _, t := range plan.Bundle.Spec.LimitTemplates {
		if err := s.st.SaveTemplate(ctx, toStoreTemplate(t)); err != nil {
			return res, err
		}
		res.Applied++
		if t.Default {
			if err := s.st.SetDefaultTemplate(ctx, t.Name); err != nil {
				return res, err
			}
		}
	}
	if deleteMissing {
		for _, name := range plan.Deletions["limitTemplates"] {
			if err := s.st.DeleteTemplate(ctx, name); err != nil && !errors.Is(err, store.ErrNotFound) {
				return res, err
			}
			res.Deleted++
		}
	}
	return res, nil
}

func (s *Service) applyUsers(ctx context.Context, plan *Plan, opts ApplyOptions) (BlockResult, error) {
	res := BlockResult{Block: BlockUsers, Status: "applied", Passwords: map[string]string{}}
	for _, u := range plan.Bundle.Spec.Users {
		existing, err := s.st.GetUser(ctx, u.Name)
		exists := err == nil
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return res, err
		}
		if exists && conflictAction(opts, "users", u.Name) == ActionKeep {
			res.Skipped++
			continue
		}
		if !exists {
			hash := u.PasswordHash
			if hash == "" {
				password, err := randomPassword()
				if err != nil {
					return res, err
				}
				hash, err = hashPassword(password)
				if err != nil {
					return res, err
				}
				res.Passwords[u.Name] = password
			}
			if err := s.st.CreateUser(ctx, store.User{
				Name: u.Name, Hash: hash, Role: store.Role(u.Role), Disabled: u.Disabled,
			}); err != nil {
				return res, err
			}
			res.Created++
			continue
		}
		if u.PasswordHash != "" && existing.Hash != u.PasswordHash {
			if err := s.st.SetPassword(ctx, u.Name, u.PasswordHash); err != nil {
				return res, err
			}
		}
		if string(existing.Role) != u.Role {
			if err := s.st.SetRole(ctx, u.Name, store.Role(u.Role)); err != nil {
				return res, err
			}
		}
		if existing.Disabled != u.Disabled {
			if err := s.st.SetDisabled(ctx, u.Name, u.Disabled); err != nil {
				return res, err
			}
		}
		res.Updated++
	}
	if opts.DeleteMissing {
		for _, name := range plan.Deletions["users"] {
			if err := s.st.DeleteUser(ctx, name); err != nil && !errors.Is(err, store.ErrNotFound) {
				return res, err
			}
			res.Deleted++
		}
	}
	if admins, err := s.activeAdmins(ctx); err != nil {
		return res, err
	} else if admins == 0 {
		return res, errors.New("bad_request: не осталось активного администратора")
	}
	return res, nil
}

func (s *Service) activeAdmins(ctx context.Context) (int, error) {
	users, err := s.st.ListUsers(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, u := range users {
		if u.Role == store.RoleAdmin && !u.Disabled {
			n++
		}
	}
	return n, nil
}

func (s *Service) applyServices(ctx context.Context, plan *Plan, opts ApplyOptions) (BlockResult, error) {
	res := BlockResult{Block: BlockServices, Status: "applied"}
	secrets := plan.Bundle.Metadata.Secrets == SecretsIncluded
	for _, svc := range plan.Bundle.Spec.Services {
		_, err := s.st.GetService(ctx, svc.Name)
		exists := err == nil
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return res, err
		}
		if exists && conflictAction(opts, "services", svc.Name) == ActionKeep {
			res.Skipped++
			continue
		}
		if !exists {
			if err := s.st.CreateService(ctx, store.ServiceAccount{
				Name: svc.Name, Description: svc.Description,
				Scope: store.ServiceScope(svc.Scope), Disabled: svc.Disabled,
			}); err != nil && !errors.Is(err, store.ErrExists) {
				return res, err
			}
			res.Created++
		} else {
			desc := svc.Description
			scope := store.ServiceScope(svc.Scope)
			disabled := svc.Disabled
			if _, err := s.st.UpdateService(ctx, svc.Name, store.ServiceUpdate{
				Description: &desc, Scope: &scope, Disabled: &disabled,
			}); err != nil {
				return res, err
			}
			res.Updated++
		}
		if secrets {
			for _, t := range svc.Tokens {
				createdAt := s.now().UTC()
				if parsed, err := parseTime(t.CreatedAt); err == nil {
					createdAt = parsed
				}
				var expires *time.Time
				if t.ExpiresAt != nil {
					if parsed, err := parseTime(*t.ExpiresAt); err == nil {
						expires = &parsed
					}
				}
				if err := s.st.ImportServiceToken(ctx, store.ServiceTokenSecret{
					ID: t.ID, Service: svc.Name, Label: t.Label, CreatedBy: t.CreatedBy,
					SecretHash: t.SecretHash, CreatedAt: createdAt, ExpiresAt: expires,
				}); err != nil {
					return res, err
				}
			}
		}
	}
	if opts.DeleteMissing {
		for _, name := range plan.Deletions["services"] {
			if err := s.st.DeleteService(ctx, name); err != nil && !errors.Is(err, store.ErrNotFound) {
				return res, err
			}
			res.Deleted++
		}
	}
	return res, nil
}

func (s *Service) applyClients(ctx context.Context, plan *Plan, opts ApplyOptions) (BlockResult, error) {
	if s.telemt == nil {
		return BlockResult{}, errors.New("proxy_unavailable: TELEMT_API_URL не задан")
	}
	res := BlockResult{Block: BlockClients, Status: "applied", Links: map[string][]string{}}
	secrets := plan.Bundle.Metadata.Secrets == SecretsIncluded
	existing, err := s.telemt.ListUsers(ctx)
	if err != nil {
		return res, err
	}
	byName := map[string]telemt.User{}
	for _, u := range existing {
		byName[u.Username] = u
	}
	for _, c := range plan.Bundle.Spec.Clients {
		_, exists := byName[c.Username]
		if exists && conflictAction(opts, "clients", c.Username) == ActionKeep {
			res.Skipped++
			continue
		}
		if !exists {
			in := telemt.CreateInput{
				Username: c.Username, MaxTCPConns: c.MaxTcpConns, MaxUniqueIPs: c.MaxUniqueIps,
				ExpirationRFC3339: c.ExpirationRFC3339, DataQuotaBytes: c.DataQuotaBytes,
				QuotaPeriod: c.QuotaPeriod, RateLimitUpBps: c.RateLimitUpBps,
				RateLimitDownBps: c.RateLimitDownBps,
			}
			enabled := c.Enabled
			in.Enabled = &enabled
			if secrets && c.Secret != "" {
				in.Secret = c.Secret
			}
			created, err := s.telemt.CreateUser(ctx, in)
			if err != nil {
				return res, err
			}
			if created != nil && created.Links != nil {
				res.Links[c.Username] = allLinks(created.Links)
			}
			res.Created++
		} else {
			patch := map[string]any{
				"max_tcp_conns": c.MaxTcpConns, "max_unique_ips": c.MaxUniqueIps,
				"expiration_rfc3339": c.ExpirationRFC3339, "data_quota_bytes": c.DataQuotaBytes,
				"quota_period": c.QuotaPeriod, "rate_limit_up_bps": c.RateLimitUpBps,
				"rate_limit_down_bps": c.RateLimitDownBps, "enabled": c.Enabled,
			}
			if secrets && c.Secret != "" {
				patch["secret"] = c.Secret
			}
			if err := s.telemt.PatchUser(ctx, c.Username, patch); err != nil {
				return res, err
			}
			res.Updated++
		}
		if c.Note != "" || exists {
			_ = s.st.SetNote(ctx, c.Username, c.Note)
		}
		if c.Trusted != nil {
			_ = s.st.SetTrusted(ctx, c.Username, store.Trusted{Countries: c.Trusted.Countries, IPs: c.Trusted.IPs})
		}
		if c.Risk != nil {
			_ = s.st.SetRisk(ctx, c.Username, store.Risk{
				WarnIPs: c.Risk.WarnIps, HighIPs: c.Risk.HighIps,
				Distinct7d: c.Risk.Distinct7d, NewCountries: c.Risk.NewCountries,
			})
		}
	}
	if opts.DeleteMissing {
		for _, name := range plan.Deletions["clients"] {
			if err := s.telemt.DeleteUser(ctx, name); err != nil {
				var apiErr *telemt.APIError
				if !errors.As(err, &apiErr) || apiErr.Code != telemt.CodeNotFound {
					return res, err
				}
			}
			_ = s.st.DeleteRisk(ctx, name)
			_ = s.st.SetNote(ctx, name, "")
			_ = s.st.SetTrusted(ctx, name, store.Trusted{})
			res.Deleted++
		}
	}
	return res, nil
}

func (s *Service) applyProxy(ctx context.Context, plan *Plan) (BlockResult, error) {
	if s.telemt == nil {
		return BlockResult{}, errors.New("proxy_unavailable: TELEMT_API_URL не задан")
	}
	cfg, err := s.telemt.GetConfig(ctx)
	if err != nil {
		return BlockResult{}, err
	}
	if plan.TargetProxyRevision != "" && cfg.Revision != plan.TargetProxyRevision {
		return BlockResult{}, ErrStalePlan
	}
	if err := s.telemt.ValidateConfig(ctx, plan.Bundle.Spec.Proxy.Config); err != nil {
		return BlockResult{}, err
	}
	res, err := s.telemt.PatchConfig(ctx, plan.Bundle.Spec.Proxy.Config, cfg.Revision, true)
	if err != nil {
		return BlockResult{}, err
	}
	out := BlockResult{Block: BlockProxy, Status: "applied", Applied: len(res.ChangedPaths)}
	if res.ReloadID != 0 {
		out.Applied++
	}
	return out, nil
}

func conflictAction(opts ApplyOptions, block, name string) string {
	if opts.Conflicts == nil {
		return ActionKeep
	}
	if action := opts.Conflicts[block][name]; action == ActionOverwrite {
		return ActionOverwrite
	}
	return ActionKeep
}

func hashPassword(password string) (string, error) { return auth.Hash(password) }

func parseTime(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, errors.New("пустое время")
	}
	return time.Parse(time.RFC3339, raw)
}

func randomPassword() (string, error) {
	raw := make([]byte, 18)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func allLinks(l *telemt.Links) []string {
	out := make([]string, 0, len(l.TLS)+len(l.Secure)+len(l.Classic))
	out = append(out, l.TLS...)
	out = append(out, l.Secure...)
	out = append(out, l.Classic...)
	return out
}
