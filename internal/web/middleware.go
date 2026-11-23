package web

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/McConfig/telemt-admin/internal/store"
)

type ctxKey int

const sessionCtxKey ctxKey = 1

func clientAddr(r *http.Request) (netip.Addr, bool) {
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

func (s *Server) allowlist(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addr, ok := clientAddr(r)
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
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; form-action 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

const cookieName = "tgadmin_session"

func (s *Server) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil {
			s.redirectLogin(w, r)
			return
		}
		sess, err := s.store.GetSession(r.Context(), c.Value)
		if err != nil {
			s.clearCookie(w)
			s.redirectLogin(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), sessionCtxKey, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) currentSession(r *http.Request) *store.Session {
	sess, _ := r.Context().Value(sessionCtxKey).(*store.Session)
	return sess
}
