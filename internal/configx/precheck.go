package configx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/McConfig/telemt-admin/internal/settings"
	"github.com/McConfig/telemt-admin/internal/store"
)

var (
	nameRe   = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	secretRe = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// PrecheckOptions — входные данные предпросмотра.
type PrecheckOptions struct {
	FileName              string
	FileHash              string
	Bundle                *Bundle
	CurrentUser           string
	TargetSettingsVersion int64
	TargetProxyRevision   string
}

var proxySections = map[string]bool{
	"general": true, "timeouts": true, "censorship": true, "upstreams": true,
	"dc_overrides": true, "web": true, "network": true, "server": true,
}

// Precheck проверяет файл и строит план без побочных эффектов.
func (s *Service) Precheck(ctx context.Context, opts PrecheckOptions) (*Plan, error) {
	b := opts.Bundle
	if b == nil {
		return nil, errors.New("bad_request: нет бандла")
	}
	plan := &Plan{
		FileName: opts.FileName, FileHash: opts.FileHash,
		Bundle: b, TargetSettingsVersion: opts.TargetSettingsVersion,
		TargetProxyRevision: opts.TargetProxyRevision,
		Diff:                map[string][]DiffEntry{},
		Conflicts:           map[string][]ConflictItem{},
		Deletions:           map[string][]string{},
	}
	if b.HasBlock(BlockSettings) {
		if err := s.precheckSettings(ctx, b, plan); err != nil {
			return nil, err
		}
	}
	if b.HasBlock(BlockTemplates) {
		if err := s.precheckTemplates(ctx, b, plan); err != nil {
			return nil, err
		}
	}
	if b.HasBlock(BlockUsers) {
		if err := s.precheckUsers(ctx, b, plan, opts.CurrentUser); err != nil {
			return nil, err
		}
	}
	if b.HasBlock(BlockServices) {
		if err := s.precheckServices(ctx, b, plan); err != nil {
			return nil, err
		}
	}
	if b.HasBlock(BlockClients) {
		if err := s.precheckClients(ctx, b, plan); err != nil {
			return nil, err
		}
	}
	if b.HasBlock(BlockProxy) {
		if err := s.precheckProxy(ctx, b, plan); err != nil {
			return nil, err
		}
	}
	return plan, nil
}

func (s *Service) precheckSettings(ctx context.Context, b *Bundle, plan *Plan) error {
	values := map[string]json.RawMessage{}
	for section, fields := range b.Spec.Settings {
		if section == "infrastructure" {
			return errors.New("bad_request: раздел infrastructure не импортируется")
		}
		for key, value := range fields {
			full := key
			if !strings.HasPrefix(key, section+".") {
				full = section + "." + key
			}
			raw, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("bad_yaml: %s: %w", full, err)
			}
			values[full] = raw
		}
	}
	if _, err := s.settings.Validate(ctx, values, nil); err != nil {
		return err
	}
	current := map[string]any{}
	for _, f := range s.settings.ExportValues(ctx) {
		current[f.Key] = f.Value
	}
	for _, path := range sortedKeys(values) {
		var after any
		_ = json.Unmarshal(values[path], &after)
		before := current[path]
		if !reflect.DeepEqual(before, after) {
			plan.Diff["settings"] = append(plan.Diff["settings"], DiffEntry{Path: path, Before: before, After: after})
		}
	}
	plan.Blocks = append(plan.Blocks, BlockPlan{ID: BlockSettings, Title: "Системные настройки", Changes: len(plan.Diff["settings"])})
	return nil
}

func (s *Service) precheckTemplates(ctx context.Context, b *Bundle, plan *Plan) error {
	existing, err := s.st.ListTemplates(ctx)
	if err != nil {
		return err
	}
	byName := map[string]store.LimitTemplate{}
	names := make([]string, 0, len(existing))
	for _, t := range existing {
		byName[t.Name] = t
		names = append(names, t.Name)
	}
	seen := map[string]bool{}
	for _, t := range b.Spec.LimitTemplates {
		if err := settings.ValidateTemplate(toStoreTemplate(t)); err != nil {
			return err
		}
		seen[t.Name] = true
	}
	for _, name := range names {
		if !seen[name] {
			plan.Deletions["limitTemplates"] = append(plan.Deletions["limitTemplates"], name)
		}
	}
	plan.Blocks = append(plan.Blocks, BlockPlan{ID: BlockTemplates, Title: "Шаблоны лимитов", Changes: len(b.Spec.LimitTemplates)})
	return nil
}

func (s *Service) precheckUsers(ctx context.Context, b *Bundle, plan *Plan, currentUser string) error {
	existing, err := s.st.ListUsers(ctx)
	if err != nil {
		return err
	}
	byName := map[string]store.User{}
	for _, u := range existing {
		byName[u.Name] = u
	}
	seen := map[string]bool{}
	result := map[string]store.User{}
	for name, u := range byName {
		result[name] = u
	}
	for _, u := range b.Spec.Users {
		if !nameRe.MatchString(u.Name) {
			return fmt.Errorf("bad_request: недопустимое имя пользователя %q", u.Name)
		}
		if u.Role != string(store.RoleAdmin) && u.Role != string(store.RoleViewer) {
			return fmt.Errorf("bad_request: роль %q должна быть admin или viewer", u.Role)
		}
		if u.PasswordHash != "" && !strings.HasPrefix(u.PasswordHash, "$2") {
			return fmt.Errorf("bad_request: passwordHash для %q не bcrypt", u.Name)
		}
		seen[u.Name] = true
		old, exists := byName[u.Name]
		changes := userChanges(old, u, exists)
		if exists && len(changes) > 0 {
			plan.Conflicts["users"] = append(plan.Conflicts["users"], ConflictItem{Name: u.Name, Action: ActionKeep, Changes: changes})
		}
		if u.Name == currentUser && (u.Disabled || u.Role != string(store.RoleAdmin)) {
			return errors.New("bad_request: нельзя отключить или понизить текущего администратора")
		}
		u2 := old
		u2.Name = u.Name
		u2.Role = store.Role(u.Role)
		u2.Disabled = u.Disabled
		if u.PasswordHash != "" {
			u2.Hash = u.PasswordHash
		}
		result[u.Name] = u2
		if exists {
			plan.Diff["users"] = append(plan.Diff["users"], DiffEntry{Path: "users." + u.Name, Before: old.Role, After: u.Role})
		}
	}
	for name := range byName {
		if !seen[name] {
			plan.Deletions["users"] = append(plan.Deletions["users"], name)
		}
	}
	admins := 0
	for _, u := range result {
		if u.Role == store.RoleAdmin && !u.Disabled {
			admins++
		}
	}
	if admins == 0 {
		return errors.New("bad_request: после импорта не останется активного администратора")
	}
	plan.Blocks = append(plan.Blocks, BlockPlan{ID: BlockUsers, Title: "Пользователи UI", Changes: len(b.Spec.Users)})
	return nil
}

func (s *Service) precheckServices(ctx context.Context, b *Bundle, plan *Plan) error {
	existing, err := s.st.ListServices(ctx)
	if err != nil {
		return err
	}
	byName := map[string]store.ServiceAccount{}
	for _, svc := range existing {
		byName[svc.Name] = svc
	}
	seen := map[string]bool{}
	for _, svc := range b.Spec.Services {
		if !nameRe.MatchString(svc.Name) {
			return fmt.Errorf("bad_request: недопустимое имя сервиса %q", svc.Name)
		}
		switch store.ServiceScope(svc.Scope) {
		case store.ScopeRead, store.ScopeWrite, store.ScopeAdmin:
		default:
			return fmt.Errorf("bad_request: scope %q недопустим", svc.Scope)
		}
		for _, t := range svc.Tokens {
			if t.ID == "" || t.SecretHash == "" {
				return fmt.Errorf("bad_request: токен сервиса %q без id или хеша", svc.Name)
			}
		}
		seen[svc.Name] = true
		if old, ok := byName[svc.Name]; ok {
			changes := serviceChanges(old, svc)
			if len(changes) > 0 {
				plan.Conflicts["services"] = append(plan.Conflicts["services"], ConflictItem{Name: svc.Name, Action: ActionKeep, Changes: changes})
			}
		}
	}
	for name := range byName {
		if !seen[name] {
			plan.Deletions["services"] = append(plan.Deletions["services"], name)
		}
	}
	plan.Blocks = append(plan.Blocks, BlockPlan{ID: BlockServices, Title: "Сервисные аккаунты", Changes: len(b.Spec.Services)})
	return nil
}

func (s *Service) precheckClients(ctx context.Context, b *Bundle, plan *Plan) error {
	if s.telemt == nil {
		return errors.New("proxy_unavailable: TELEMT_API_URL не задан")
	}
	users, err := s.telemt.ListUsers(ctx)
	if err != nil {
		return err
	}
	byName := map[string]bool{}
	for _, u := range users {
		byName[u.Username] = true
	}
	seen := map[string]bool{}
	for _, c := range b.Spec.Clients {
		if !nameRe.MatchString(c.Username) {
			return fmt.Errorf("bad_request: недопустимое имя клиента %q", c.Username)
		}
		if c.Secret != "" && !secretRe.MatchString(c.Secret) {
			return fmt.Errorf("bad_request: секрет клиента %q не 32 hex", c.Username)
		}
		seen[c.Username] = true
		if byName[c.Username] {
			changes := clientChanges(c)
			plan.Conflicts["clients"] = append(plan.Conflicts["clients"], ConflictItem{Name: c.Username, Action: ActionKeep, Changes: changes})
			if c.Secret != "" {
				plan.Diff["clients"] = append(plan.Diff["clients"], DiffEntry{Path: "clients." + c.Username + ".secret", Secret: true})
			}
		}
	}
	for name := range byName {
		if !seen[name] {
			plan.Deletions["clients"] = append(plan.Deletions["clients"], name)
		}
	}
	plan.Blocks = append(plan.Blocks, BlockPlan{ID: BlockClients, Title: "Клиенты telemt", Changes: len(b.Spec.Clients)})
	return nil
}

func (s *Service) precheckProxy(ctx context.Context, b *Bundle, plan *Plan) error {
	if s.telemt == nil {
		return errors.New("proxy_unavailable: TELEMT_API_URL не задан")
	}
	for section := range b.Spec.Proxy.Config {
		if !proxySections[section] {
			return fmt.Errorf("bad_request: секция прокси %q не импортируется", section)
		}
	}
	cfg, err := s.telemt.GetConfig(ctx)
	if err != nil {
		return err
	}
	if err := s.telemt.ValidateConfig(ctx, b.Spec.Proxy.Config); err != nil {
		return err
	}
	plan.Diff["proxy"] = recursiveDiff("", cfg.Config, b.Spec.Proxy.Config)
	if plan.TargetProxyRevision == "" {
		plan.TargetProxyRevision = cfg.Revision
	}
	plan.Blocks = append(plan.Blocks, BlockPlan{ID: BlockProxy, Title: "Конфигурация прокси", Changes: len(plan.Diff["proxy"])})
	return nil
}

func toStoreTemplate(t LimitTemplate) store.LimitTemplate {
	return store.LimitTemplate{
		Name: t.Name, MaxIps: t.MaxIps, MaxTcpConns: t.MaxTcpConns,
		DataQuotaBytes: t.DataQuotaBytes, QuotaPeriod: t.QuotaPeriod,
		RateLimitUpBps: t.RateLimitUpBps, RateLimitDownBps: t.RateLimitDownBps,
		ExpirationDays: t.ExpirationDays, SourceDeny: t.SourceDeny,
		RiskWarnIps: t.RiskWarnIps, RiskHighIps: t.RiskHighIps,
		RiskDistinct7d: t.RiskDistinct7d, RiskNewCountries: t.RiskNewCountries,
	}
}

func userChanges(old store.User, u User, exists bool) []string {
	if !exists {
		return nil
	}
	out := []string{}
	if string(old.Role) != u.Role {
		out = append(out, "role")
	}
	if old.Disabled != u.Disabled {
		out = append(out, "disabled")
	}
	if u.PasswordHash != "" && old.Hash != u.PasswordHash {
		out = append(out, "password")
	}
	return out
}

func serviceChanges(old store.ServiceAccount, svc ServiceAccount) []string {
	out := []string{}
	if string(old.Scope) != svc.Scope {
		out = append(out, "scope")
	}
	if old.Disabled != svc.Disabled {
		out = append(out, "disabled")
	}
	if old.Description != svc.Description {
		out = append(out, "description")
	}
	return out
}

func clientChanges(c Client) []string {
	out := []string{}
	if c.MaxTcpConns != nil {
		out = append(out, "maxTcpConns")
	}
	if c.MaxUniqueIps != nil {
		out = append(out, "maxUniqueIps")
	}
	if c.DataQuotaBytes != nil {
		out = append(out, "dataQuotaBytes")
	}
	if c.Note != "" {
		out = append(out, "note")
	}
	return out
}

func recursiveDiff(prefix string, before, after map[string]any) []DiffEntry {
	out := []DiffEntry{}
	keys := map[string]bool{}
	for k := range before {
		keys[k] = true
	}
	for k := range after {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		bv, bok := before[k]
		av, aok := after[k]
		bm, bIsMap := bv.(map[string]any)
		am, aIsMap := av.(map[string]any)
		if bIsMap && aIsMap {
			out = append(out, recursiveDiff(path, bm, am)...)
			continue
		}
		if !bok || !aok || !reflect.DeepEqual(bv, av) {
			out = append(out, DiffEntry{Path: path, Before: bv, After: av})
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
