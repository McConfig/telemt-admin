package store

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

func seenKey(user string) string          { return "iphist:seen:" + user }
func firstKey(user string) string         { return "iphist:first:" + user }
func ipKey(user, ip string) string        { return "iphist:ip:" + user + ":" + ip }
func dayKey(user, day string) string      { return "iphist:day:" + user + ":" + day }
func dayFirstKey(user, day string) string { return "iphist:dayfirst:" + user + ":" + day }
func dayLastKey(user, day string) string  { return "iphist:daylast:" + user + ":" + day }
func maxKey(user, day string) string      { return "iphist:max:" + user + ":" + day }
func trafficKey(user string) string       { return "iphist:traffic:" + user }
func trafficDaysKey(user string) string   { return "iphist:trafficdays:" + user }
func geoKey(user string) string           { return "iphist:geo:" + user }
func countryKey(user, day string) string  { return "iphist:country:" + user + ":" + day }
func foreignKey(user, day string) string  { return "iphist:foreign:" + user + ":" + day }
func countryFirstKey(user string) string  { return "iphist:countryfirst:" + user }

// IPStat — один адрес в истории пользователя.
type IPStat struct {
	IP          string
	FirstSeen   time.Time
	LastSeen    time.Time
	Count       int64
	Country     string
	CountryName string
	ASN         uint32
	ASNOrg      string
	Internal    bool
}

// IPGeo — сведения GeoIP, которые сохраняются вместе с адресом.
type IPGeo struct {
	Country     string
	CountryName string
	ASN         uint32
	ASNOrg      string
	Internal    bool
}

func (g IPGeo) empty() bool {
	return g.Country == "" && g.ASN == 0 && g.ASNOrg == "" && !g.Internal
}

// GeoStats — последние агрегаты GeoIP по активным адресам пользователя.
type GeoStats struct {
	ForeignCurrent int64
	CountryCodes   []string
	Countries      []string
	Operators      []string
}

// SaveGeoStats сохраняет агрегаты GeoIP для карточки и списка клиентов.
func (s *Store) SaveGeoStats(ctx context.Context, user string, foreign int64, codes, names, operators []string) error {
	codesJSON, err := json.Marshal(codes)
	if err != nil {
		return err
	}
	namesJSON, err := json.Marshal(names)
	if err != nil {
		return err
	}
	operatorsJSON, err := json.Marshal(operators)
	if err != nil {
		return err
	}
	if err := s.rdb.HSet(ctx, geoKey(user), map[string]any{
		"foreign": foreign, "codes": string(codesJSON), "names": string(namesJSON),
		"operators": string(operatorsJSON),
	}).Err(); err != nil {
		return err
	}
	return s.rdb.Expire(ctx, geoKey(user), s.Retention()).Err()
}

// GeoStats читает последние агрегаты GeoIP. Второй результат — есть ли запись.
func (s *Store) GeoStats(ctx context.Context, user string) (GeoStats, bool, error) {
	m, err := s.rdb.HGetAll(ctx, geoKey(user)).Result()
	if err != nil {
		return GeoStats{}, false, err
	}
	if len(m) == 0 {
		return GeoStats{}, false, nil
	}
	var out GeoStats
	if n, err := strconv.ParseInt(m["foreign"], 10, 64); err == nil {
		out.ForeignCurrent = n
	}
	_ = json.Unmarshal([]byte(m["codes"]), &out.CountryCodes)
	_ = json.Unmarshal([]byte(m["names"]), &out.Countries)
	_ = json.Unmarshal([]byte(m["operators"]), &out.Operators)
	return out, true, nil
}

// UserIPStats — агрегаты по адресам и трафику пользователя.
type UserIPStats struct {
	Distinct24h  int64
	Distinct7d   int64
	New24h       int64
	MaxToday     int64
	TrafficToday uint64
	// Foreign24h — иностранные адреса за текущие сутки.
	Foreign24h int64
	// Countries24h и Countries7d — сколько разных стран у адресов.
	Countries24h int64
	Countries7d  int64
	// NewCountries24h — иностранные страны, которых не было раньше.
	NewCountries24h int64
	// NewCountryCodes — коды этих стран.
	NewCountryCodes []string
}

// RecordIPObservation сохраняет срез активных адресов и суммарный трафик.
// geo содержит сведения GeoIP по адресам; отсутствующие адреса просто не
// обогащаются.
func (s *Store) RecordIPObservation(ctx context.Context, user string, ips []string, geo map[string]IPGeo, total uint64, now time.Time) error {
	now = now.UTC()
	day := now.Format("2006-01-02")
	ts := float64(now.Unix())

	pipe := s.rdb.TxPipeline()
	for _, ip := range ips {
		pipe.ZAdd(ctx, seenKey(user), redis.Z{Score: ts, Member: ip})
		pipe.ZAddArgs(ctx, firstKey(user), redis.ZAddArgs{NX: true, Members: []redis.Z{{Score: ts, Member: ip}}})
		key := ipKey(user, ip)
		pipe.HSet(ctx, key, "last_seen", now.Format(time.RFC3339))
		pipe.HSetNX(ctx, key, "first_seen", now.Format(time.RFC3339))
		pipe.HIncrBy(ctx, key, "count", 1)
		if g, ok := geo[ip]; ok && !g.empty() {
			fields := map[string]any{}
			if g.Country != "" {
				fields["country"] = g.Country
				fields["country_name"] = g.CountryName
			}
			if g.ASN != 0 {
				fields["asn"] = g.ASN
			}
			if g.ASNOrg != "" {
				fields["asn_org"] = g.ASNOrg
			}
			if g.Internal {
				fields["internal"] = "1"
			}
			if len(fields) > 0 {
				pipe.HSet(ctx, key, fields)
			}
		}
		if g, ok := geo[ip]; ok && g.Country != "" && !g.Internal {
			pipe.SAdd(ctx, countryKey(user, day), g.Country)
			pipe.Expire(ctx, countryKey(user, day), s.Retention())
			if g.Country != s.HomeCountry() {
				pipe.SAdd(ctx, foreignKey(user, day), ip)
				pipe.Expire(ctx, foreignKey(user, day), s.Retention())
				pipe.ZAddArgs(ctx, countryFirstKey(user), redis.ZAddArgs{
					NX: true, Members: []redis.Z{{Score: ts, Member: g.Country}},
				})
				pipe.Expire(ctx, countryFirstKey(user), s.Retention())
			}
		}
		pipe.Expire(ctx, key, s.Retention())
		pipe.SAdd(ctx, dayKey(user, day), ip)
		pipe.HSetNX(ctx, dayFirstKey(user, day), ip, now.Format(time.RFC3339))
		pipe.HSet(ctx, dayLastKey(user, day), ip, now.Format(time.RFC3339))
	}
	pipe.Expire(ctx, dayKey(user, day), s.Retention())
	pipe.Expire(ctx, dayFirstKey(user, day), s.Retention())
	pipe.Expire(ctx, dayLastKey(user, day), s.Retention())
	pipe.Expire(ctx, seenKey(user), s.Retention())
	pipe.Expire(ctx, firstKey(user), s.Retention())
	pipe.ZRemRangeByScore(ctx, seenKey(user), "-inf", strconv.FormatInt(now.Add(-s.Retention()).Unix(), 10))
	pipe.ZRemRangeByScore(ctx, firstKey(user), "-inf", strconv.FormatInt(now.Add(-s.Retention()).Unix(), 10))
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}

	if len(ips) > 0 {
		max, err := s.rdb.Get(ctx, maxKey(user, day)).Int64()
		if err != nil && err != redis.Nil {
			return err
		}
		if int64(len(ips)) > max {
			if err := s.rdb.Set(ctx, maxKey(user, day), len(ips), s.Retention()).Err(); err != nil {
				return err
			}
		}
	}
	return s.recordTraffic(ctx, user, total, day, now)
}

func (s *Store) recordTraffic(ctx context.Context, user string, total uint64, day string, now time.Time) error {
	m, err := s.rdb.HGetAll(ctx, trafficKey(user)).Result()
	if err != nil {
		return err
	}
	if len(m) == 0 {
		if err := s.rdb.HSet(ctx, trafficKey(user), map[string]any{
			"day": day, "day_bytes": 0, "last_total": total,
		}).Err(); err != nil {
			return err
		}
		if err := s.rdb.HSet(ctx, trafficDaysKey(user), day, 0).Err(); err != nil {
			return err
		}
		_ = s.rdb.Expire(ctx, trafficDaysKey(user), s.Retention()).Err()
		return nil
	}
	last, _ := strconv.ParseUint(m["last_total"], 10, 64)
	dayBytes, _ := strconv.ParseUint(m["day_bytes"], 10, 64)
	storedDay := m["day"]
	var delta uint64
	if total >= last {
		delta = total - last
	} else {
		delta = total
	}
	if storedDay != day {
		dayBytes = delta
	} else {
		dayBytes += delta
	}
	if err := s.rdb.HSet(ctx, trafficKey(user), map[string]any{
		"day": day, "day_bytes": dayBytes, "last_total": total,
	}).Err(); err != nil {
		return err
	}
	if err := s.rdb.HSet(ctx, trafficDaysKey(user), day, dayBytes).Err(); err != nil {
		return err
	}
	_ = s.rdb.Expire(ctx, trafficKey(user), s.Retention()).Err()
	_ = s.rdb.Expire(ctx, trafficDaysKey(user), s.Retention()).Err()
	_ = now
	return nil
}

// UserIPStats считает агрегаты за окна и трафик за текущие сутки.
func (s *Store) UserIPStats(ctx context.Context, user string, now time.Time) (UserIPStats, error) {
	now = now.UTC()
	day := now.Format("2006-01-02")
	var out UserIPStats
	var err error
	if out.Distinct24h, err = s.rdb.ZCount(ctx, seenKey(user),
		strconv.FormatInt(now.Add(-24*time.Hour).Unix(), 10), "+inf").Result(); err != nil {
		return out, err
	}
	if out.Distinct7d, err = s.rdb.ZCount(ctx, seenKey(user),
		strconv.FormatInt(now.Add(-7*24*time.Hour).Unix(), 10), "+inf").Result(); err != nil {
		return out, err
	}
	if out.New24h, err = s.rdb.ZCount(ctx, firstKey(user),
		strconv.FormatInt(now.Add(-24*time.Hour).Unix(), 10), "+inf").Result(); err != nil {
		return out, err
	}
	max, err := s.rdb.Get(ctx, maxKey(user, day)).Int64()
	if err != nil && err != redis.Nil {
		return out, err
	}
	out.MaxToday = max
	m, err := s.rdb.HGetAll(ctx, trafficKey(user)).Result()
	if err != nil {
		return out, err
	}
	if m["day"] == day {
		out.TrafficToday, _ = strconv.ParseUint(m["day_bytes"], 10, 64)
	}
	out.Foreign24h, err = s.rdb.SCard(ctx, foreignKey(user, day)).Result()
	if err != nil {
		return out, err
	}
	out.Countries24h, err = s.rdb.SCard(ctx, countryKey(user, day)).Result()
	if err != nil {
		return out, err
	}
	week := make([]string, 0, 7)
	for i := 0; i < 7; i++ {
		week = append(week, countryKey(user, now.AddDate(0, 0, -i).Format("2006-01-02")))
	}
	countries, err := s.rdb.SUnion(ctx, week...).Result()
	if err != nil {
		return out, err
	}
	out.Countries7d = int64(len(countries))

	// Новые страны считаем только у клиентов, которые уже наблюдаются больше
	// суток: иначе в первый день всё будет «новым».
	first, err := s.rdb.ZRangeWithScores(ctx, firstKey(user), 0, 0).Result()
	if err != nil {
		return out, err
	}
	if len(first) > 0 && int64(first[0].Score) <= now.Add(-24*time.Hour).Unix() {
		codes, err := s.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
			Key: countryFirstKey(user), Start: strconv.FormatInt(now.Add(-24*time.Hour).Unix(), 10),
			Stop: "+inf", ByScore: true,
		}).Result()
		if err != nil {
			return out, err
		}
		out.NewCountryCodes = codes
		out.NewCountries24h = int64(len(codes))
	}
	return out, nil
}

// TrafficDay — трафик пользователя за календарный день.
type TrafficDay struct {
	Day   string
	Bytes uint64
}

// TrafficHistory возвращает трафик по дням за последние days дней, старые
// записи первыми. Дни без данных не попадают в ответ.
func (s *Store) TrafficHistory(ctx context.Context, user string, days int, now time.Time) ([]TrafficDay, error) {
	if days <= 0 || days > 90 {
		days = 30
	}
	m, err := s.rdb.HGetAll(ctx, trafficDaysKey(user)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]TrafficDay, 0, days)
	for i := days - 1; i >= 0; i-- {
		day := now.UTC().AddDate(0, 0, -i).Format("2006-01-02")
		if raw, ok := m[day]; ok {
			bytes, _ := strconv.ParseUint(raw, 10, 64)
			out = append(out, TrafficDay{Day: day, Bytes: bytes})
		}
	}
	return out, nil
}

// DayActivity — активность пользователя за день для таймлайна.
type DayActivity struct {
	Day          string
	NewIPs       int
	NewCountries []string
	MaxIPs       int64
	Bytes        uint64
}

// Timeline собирает активность по дням: новые адреса, новые страны, пик
// одновременных адресов и трафик. Дни без событий не попадают в ответ.
func (s *Store) Timeline(ctx context.Context, user string, days int, now time.Time) ([]DayActivity, error) {
	if days <= 0 || days > 90 {
		days = 14
	}
	now = now.UTC()
	byDay := make(map[string]*DayActivity, days)
	for i := days - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -i).Format("2006-01-02")
		byDay[day] = &DayActivity{Day: day}
	}
	history, err := s.IPHistory(ctx, user)
	if err != nil {
		return nil, err
	}
	for _, item := range history {
		day := item.FirstSeen.UTC().Format("2006-01-02")
		if a, ok := byDay[day]; ok {
			a.NewIPs++
		}
	}
	countries, err := s.rdb.ZRangeWithScores(ctx, countryFirstKey(user), 0, -1).Result()
	if err != nil {
		return nil, err
	}
	for _, c := range countries {
		code, ok := c.Member.(string)
		if !ok {
			continue
		}
		day := time.Unix(int64(c.Score), 0).UTC().Format("2006-01-02")
		if a, ok := byDay[day]; ok {
			a.NewCountries = append(a.NewCountries, code)
		}
	}
	traffic, err := s.rdb.HGetAll(ctx, trafficDaysKey(user)).Result()
	if err != nil {
		return nil, err
	}
	for day, raw := range traffic {
		if a, ok := byDay[day]; ok {
			a.Bytes, _ = strconv.ParseUint(raw, 10, 64)
		}
	}
	for day, a := range byDay {
		if max, err := s.rdb.Get(ctx, maxKey(user, day)).Int64(); err == nil {
			a.MaxIPs = max
		}
	}
	out := make([]DayActivity, 0, days)
	for i := days - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -i).Format("2006-01-02")
		a := byDay[day]
		if a.NewIPs == 0 && len(a.NewCountries) == 0 && a.MaxIPs == 0 && a.Bytes == 0 {
			continue
		}
		out = append(out, *a)
	}
	return out, nil
}

// IPHistory возвращает адреса пользователя, новые сверху.
func (s *Store) IPHistory(ctx context.Context, user string) ([]IPStat, error) {
	members, err := s.rdb.ZRangeWithScores(ctx, seenKey(user), 0, -1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]IPStat, 0, len(members))
	for _, m := range members {
		ip, ok := m.Member.(string)
		if !ok {
			continue
		}
		stat := IPStat{IP: ip, LastSeen: time.Unix(int64(m.Score), 0).UTC()}
		if first, err := s.rdb.ZScore(ctx, firstKey(user), ip).Result(); err == nil {
			stat.FirstSeen = time.Unix(int64(first), 0).UTC()
		}
		if fields, err := s.rdb.HGetAll(ctx, ipKey(user, ip)).Result(); err == nil {
			if n, err := strconv.ParseInt(fields["count"], 10, 64); err == nil {
				stat.Count = n
			}
			stat.Country = fields["country"]
			stat.CountryName = fields["country_name"]
			stat.ASNOrg = fields["asn_org"]
			stat.Internal = fields["internal"] == "1"
			if n, err := strconv.ParseUint(fields["asn"], 10, 32); err == nil {
				stat.ASN = uint32(n)
			}
		}
		out = append(out, stat)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	return out, nil
}

// SetIPGeo дописывает сведения GeoIP к уже сохранённому адресу.
func (s *Store) SetIPGeo(ctx context.Context, user, ip string, g IPGeo) error {
	key := ipKey(user, ip)
	fields := map[string]any{}
	if g.Country != "" {
		fields["country"] = g.Country
		fields["country_name"] = g.CountryName
	}
	if g.ASN != 0 {
		fields["asn"] = g.ASN
	}
	if g.ASNOrg != "" {
		fields["asn_org"] = g.ASNOrg
	}
	if g.Internal {
		fields["internal"] = "1"
	}
	if len(fields) == 0 {
		return nil
	}
	return s.rdb.HSet(ctx, key, fields).Err()
}

// DeleteIPHistory удаляет историю адресов и трафика пользователя.
func (s *Store) DeleteIPHistory(ctx context.Context, user string) error {
	keys := []string{seenKey(user), firstKey(user), trafficKey(user), trafficDaysKey(user), geoKey(user), countryFirstKey(user)}
	for _, pattern := range []string{
		"iphist:ip:" + user + ":*",
		"iphist:day:" + user + ":*",
		"iphist:dayfirst:" + user + ":*",
		"iphist:daylast:" + user + ":*",
		"iphist:max:" + user + ":*",
		"iphist:country:" + user + ":*",
		"iphist:foreign:" + user + ":*",
	} {
		iter := s.rdb.Scan(ctx, 0, pattern, 200).Iterator()
		for iter.Next(ctx) {
			keys = append(keys, iter.Val())
		}
		if err := iter.Err(); err != nil {
			return err
		}
	}
	if len(keys) == 0 {
		return nil
	}
	return s.rdb.Del(ctx, keys...).Err()
}

// LastSeen возвращает время последнего наблюдения любого адреса.
func (s *Store) LastSeen(ctx context.Context, user string) (time.Time, bool) {
	res, err := s.rdb.ZRevRangeWithScores(ctx, seenKey(user), 0, 0).Result()
	if err != nil || len(res) == 0 {
		return time.Time{}, false
	}
	return time.Unix(int64(res[0].Score), 0).UTC(), true
}
