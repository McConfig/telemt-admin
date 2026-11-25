package store

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RiskDayIP — адрес, активный в конкретный день.
type RiskDayIP struct {
	IP          string
	Country     string
	CountryName string
	ASN         uint32
	ASNOrg      string
	Internal    bool
	FirstSeen   time.Time
	LastSeen    time.Time
	// DayFirst и DayLast — первое и последнее наблюдение внутри дня.
	// Нулевые значения означают, что детальные времена не собирались.
	DayFirst time.Time
	DayLast  time.Time
	Count    int64
	New      bool
}

// RiskDay — день для отчёта по риску.
type RiskDay struct {
	Day          string
	Distinct     int64
	Max          int64
	NewIPs       int64
	NewCountries []string
	Traffic      uint64
	IPs          []RiskDayIP
}

// RiskDays собирает отчёт по дням: адреса, пик, новые адреса и страны,
// трафик. Дни идут от нового к старому.
func (s *Store) RiskDays(ctx context.Context, user string, days int, now time.Time) ([]RiskDay, error) {
	if days <= 0 || days > 30 {
		days = 7
	}
	now = now.UTC()

	history, err := s.IPHistory(ctx, user)
	if err != nil {
		return nil, err
	}
	byIP := make(map[string]IPStat, len(history))
	for _, item := range history {
		byIP[item.IP] = item
	}

	// Новые страны берём из countryFirstKey: это страны, впервые увиденные
	// в конкретный день, а не все страны дня.
	countryFirst, err := s.rdb.ZRangeWithScores(ctx, countryFirstKey(user), 0, -1).Result()
	if err != nil {
		return nil, err
	}
	newCountriesByDay := map[string][]string{}
	for _, item := range countryFirst {
		code, ok := item.Member.(string)
		if !ok {
			continue
		}
		day := time.Unix(int64(item.Score), 0).UTC().Format("2006-01-02")
		newCountriesByDay[day] = append(newCountriesByDay[day], code)
	}

	out := make([]RiskDay, 0, days)
	for i := 0; i < days; i++ {
		day := now.AddDate(0, 0, -i).Format("2006-01-02")

		members, err := s.rdb.SMembers(ctx, dayKey(user, day)).Result()
		if err != nil {
			return nil, err
		}
		countries, err := s.rdb.SMembers(ctx, countryKey(user, day)).Result()
		if err != nil {
			return nil, err
		}
		max, err := s.rdb.Get(ctx, maxKey(user, day)).Int64()
		if err != nil && !errors.Is(err, redis.Nil) {
			// Ошибка чтения пика не должна ломать отчёт.
			max = 0
		}
		trafficRaw, _ := s.rdb.HGet(ctx, trafficDaysKey(user), day).Result()
		traffic, _ := strconv.ParseUint(trafficRaw, 10, 64)

		if len(members) == 0 && len(countries) == 0 && max == 0 && traffic == 0 {
			continue
		}

		firsts, _ := s.rdb.HGetAll(ctx, dayFirstKey(user, day)).Result()
		lasts, _ := s.rdb.HGetAll(ctx, dayLastKey(user, day)).Result()

		ips := make([]RiskDayIP, 0, len(members))
		var newIPs int64
		for _, ip := range members {
			item, ok := byIP[ip]
			if !ok {
				continue
			}
			outIP := RiskDayIP{
				IP: item.IP, Country: item.Country, CountryName: item.CountryName,
				ASN: item.ASN, ASNOrg: item.ASNOrg, Internal: item.Internal,
				FirstSeen: item.FirstSeen, LastSeen: item.LastSeen, Count: item.Count,
			}
			if raw, ok := firsts[ip]; ok {
				if t, err := time.Parse(time.RFC3339, raw); err == nil {
					outIP.DayFirst = t
				}
			}
			if raw, ok := lasts[ip]; ok {
				if t, err := time.Parse(time.RFC3339, raw); err == nil {
					outIP.DayLast = t
				}
			}
			if item.FirstSeen.UTC().Format("2006-01-02") == day {
				outIP.New = true
				newIPs++
			}
			ips = append(ips, outIP)
		}
		sort.Slice(ips, func(i, j int) bool {
			if !ips[i].DayLast.Equal(ips[j].DayLast) {
				return ips[i].DayLast.After(ips[j].DayLast)
			}
			return ips[i].IP < ips[j].IP
		})
		newCountries := append([]string(nil), newCountriesByDay[day]...)
		sort.Strings(newCountries)

		out = append(out, RiskDay{
			Day: day, Distinct: int64(len(ips)), Max: max, NewIPs: newIPs,
			NewCountries: newCountries, Traffic: traffic, IPs: ips,
		})
	}
	return out, nil
}
