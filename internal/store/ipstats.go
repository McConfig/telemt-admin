package store

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

func seenKey(user string) string     { return "iphist:seen:" + user }
func firstKey(user string) string    { return "iphist:first:" + user }
func ipKey(user, ip string) string   { return "iphist:ip:" + user + ":" + ip }
func dayKey(user, day string) string { return "iphist:day:" + user + ":" + day }
func maxKey(user, day string) string { return "iphist:max:" + user + ":" + day }
func trafficKey(user string) string  { return "iphist:traffic:" + user }

// IPStat — один адрес в истории пользователя.
type IPStat struct {
	IP        string
	FirstSeen time.Time
	LastSeen  time.Time
	Count     int64
}

// UserIPStats — агрегаты по адресам и трафику пользователя.
type UserIPStats struct {
	Distinct24h  int64
	Distinct7d   int64
	New24h       int64
	MaxToday     int64
	TrafficToday uint64
}

// RecordIPObservation сохраняет срез активных адресов и суммарный трафик.
func (s *Store) RecordIPObservation(ctx context.Context, user string, ips []string, total uint64, now time.Time) error {
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
		pipe.Expire(ctx, key, s.retention)
		pipe.SAdd(ctx, dayKey(user, day), ip)
	}
	pipe.Expire(ctx, dayKey(user, day), s.retention)
	pipe.Expire(ctx, seenKey(user), s.retention)
	pipe.Expire(ctx, firstKey(user), s.retention)
	pipe.ZRemRangeByScore(ctx, seenKey(user), "-inf", strconv.FormatInt(now.Add(-s.retention).Unix(), 10))
	pipe.ZRemRangeByScore(ctx, firstKey(user), "-inf", strconv.FormatInt(now.Add(-s.retention).Unix(), 10))
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}

	if len(ips) > 0 {
		max, err := s.rdb.Get(ctx, maxKey(user, day)).Int64()
		if err != nil && err != redis.Nil {
			return err
		}
		if int64(len(ips)) > max {
			if err := s.rdb.Set(ctx, maxKey(user, day), len(ips), s.retention).Err(); err != nil {
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
		return s.rdb.HSet(ctx, trafficKey(user), map[string]any{
			"day": day, "day_bytes": 0, "last_total": total,
		}).Err()
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
	_ = s.rdb.Expire(ctx, trafficKey(user), s.retention).Err()
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
		if count, err := s.rdb.HGet(ctx, ipKey(user, ip), "count").Int64(); err == nil {
			stat.Count = count
		}
		out = append(out, stat)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	return out, nil
}

// DeleteIPHistory удаляет историю адресов и трафика пользователя.
func (s *Store) DeleteIPHistory(ctx context.Context, user string) error {
	keys := []string{seenKey(user), firstKey(user), trafficKey(user)}
	for _, pattern := range []string{
		"iphist:ip:" + user + ":*",
		"iphist:day:" + user + ":*",
		"iphist:max:" + user + ":*",
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
