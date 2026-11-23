package store

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const auditStream = "audit:events"

// AuditEvent — запись журнала действий.
type AuditEvent struct {
	At     time.Time
	Actor  string
	Action string
	Target string
	Result string
	IP     string
}

// Audit добавляет событие в поток.
func (s *Store) Audit(ctx context.Context, e AuditEvent) error {
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	_, err := s.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: auditStream,
		MaxLen: 10000,
		Approx: true,
		Values: map[string]any{
			"at": e.At.UTC().Format(time.RFC3339), "actor": e.Actor,
			"action": e.Action, "target": e.Target, "result": e.Result, "ip": e.IP,
		},
	}).Result()
	return err
}

// RecentAudit возвращает последние n событий, новые сверху.
func (s *Store) RecentAudit(ctx context.Context, n int64) ([]AuditEvent, error) {
	msgs, err := s.rdb.XRevRangeN(ctx, auditStream, "+", "-", n).Result()
	if err != nil {
		return nil, err
	}
	out := make([]AuditEvent, 0, len(msgs))
	for _, m := range msgs {
		str := func(k string) string {
			if v, ok := m.Values[k].(string); ok {
				return v
			}
			return ""
		}
		e := AuditEvent{
			Actor: str("actor"), Action: str("action"),
			Target: str("target"), Result: str("result"), IP: str("ip"),
		}
		e.At, _ = time.Parse(time.RFC3339, str("at"))
		out = append(out, e)
	}
	return out, nil
}
