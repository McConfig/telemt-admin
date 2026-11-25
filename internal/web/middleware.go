package web

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func parseRemote(r *http.Request) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(strings.Trim(host, "[]"))
	if err != nil {
		return netip.Addr{}, false
	}
	return addr, true
}

// clientAddr учитывает X-Forwarded-For только от доверенного reverse-proxy.
// Берётся последний адрес в цепочке: роутер дописывает реальный источник.
func (s *Server) clientAddr(r *http.Request) (netip.Addr, bool) {
	addr, ok := parseRemote(r)
	if !ok || !s.cfg.TrustsProxy(addr) {
		return addr, ok
	}
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return addr, true
	}
	parts := strings.Split(xff, ",")
	last, err := netip.ParseAddr(strings.TrimSpace(parts[len(parts)-1]))
	if err != nil {
		return addr, true
	}
	return last, true
}

func (s *Server) allowlist(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addr, ok := s.clientAddr(r)
		if !ok || !s.cfg.Allows(addr) {
			http.Error(w, "доступ только из локальной сети", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; form-action 'self'; manifest-src 'self'")
		next.ServeHTTP(w, r)
	})
}
