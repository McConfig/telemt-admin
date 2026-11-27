package web

import (
	"time"
)

type rateWindow struct {
	count int
	reset time.Time
}

// allowConfigRate проверяет лимит действий пользователя.
func (s *Server) allowConfigRate(kind, user string, limit int, window time.Duration) bool {
	key := kind + ":" + user
	now := s.now()
	s.ratesMu.Lock()
	defer s.ratesMu.Unlock()
	if s.configRates == nil {
		s.configRates = map[string]*rateWindow{}
	}
	w := s.configRates[key]
	if w == nil || now.After(w.reset) {
		s.configRates[key] = &rateWindow{count: 1, reset: now.Add(window)}
		return true
	}
	if w.count >= limit {
		return false
	}
	w.count++
	return true
}
