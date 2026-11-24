package store

import (
	"context"
	"sort"
)

func trustedCountriesKey(user string) string { return "trusted:countries:" + user }
func trustedIPsKey(user string) string       { return "trusted:ips:" + user }

// Trusted — страны и адреса, которым пользователь доверяет: они остаются
// в истории, но не поднимают риск и не дают алертов.
type Trusted struct {
	Countries []string
	IPs       []string
}

// GetTrusted возвращает доверенные страны и адреса пользователя.
func (s *Store) GetTrusted(ctx context.Context, user string) (Trusted, error) {
	countries, err := s.rdb.SMembers(ctx, trustedCountriesKey(user)).Result()
	if err != nil {
		return Trusted{}, err
	}
	ips, err := s.rdb.SMembers(ctx, trustedIPsKey(user)).Result()
	if err != nil {
		return Trusted{}, err
	}
	sort.Strings(countries)
	sort.Strings(ips)
	if countries == nil {
		countries = []string{}
	}
	if ips == nil {
		ips = []string{}
	}
	return Trusted{Countries: countries, IPs: ips}, nil
}

// SetTrusted заменяет списки доверенных стран и адресов.
func (s *Store) SetTrusted(ctx context.Context, user string, t Trusted) error {
	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, trustedCountriesKey(user), trustedIPsKey(user))
	if len(t.Countries) > 0 {
		pipe.SAdd(ctx, trustedCountriesKey(user), stringsToAny(t.Countries)...)
	}
	if len(t.IPs) > 0 {
		pipe.SAdd(ctx, trustedIPsKey(user), stringsToAny(t.IPs)...)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func stringsToAny(items []string) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}
	return out
}
