package web

import (
	"net/http"
	"net/netip"
	"regexp"
	"strings"

	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/go-chi/chi/v5"
)

var countryCodeRe = regexp.MustCompile(`^[A-Z]{2}$`)

type trustedRequest struct {
	Countries []string `json:"countries"`
	IPs       []string `json:"ips"`
}

func (s *Server) apiGetTrusted(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if !nameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "недопустимое имя")
		return
	}
	trusted, err := s.store.GetTrusted(r.Context(), name)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, trustedRequest{Countries: trusted.Countries, IPs: trusted.IPs})
}

func (s *Server) apiSetTrusted(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if !nameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "недопустимое имя")
		return
	}
	var body trustedRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "некорректный JSON")
		return
	}
	countries := make([]string, 0, len(body.Countries))
	seenCountry := map[string]bool{}
	for _, raw := range body.Countries {
		code := strings.ToUpper(strings.TrimSpace(raw))
		if !countryCodeRe.MatchString(code) {
			writeError(w, http.StatusBadRequest, "bad_request", "страна должна быть кодом ISO из двух букв")
			return
		}
		if !seenCountry[code] {
			seenCountry[code] = true
			countries = append(countries, code)
		}
	}
	ips := make([]string, 0, len(body.IPs))
	seenIP := map[string]bool{}
	for _, raw := range body.IPs {
		addr, err := netip.ParseAddr(strings.TrimSpace(raw))
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "некорректный IP: "+raw)
			return
		}
		ip := addr.String()
		if !seenIP[ip] {
			seenIP[ip] = true
			ips = append(ips, ip)
		}
	}
	if len(countries) > 50 || len(ips) > 50 {
		writeError(w, http.StatusBadRequest, "bad_request", "не больше 50 стран и 50 адресов")
		return
	}
	if err := s.store.SetTrusted(r.Context(), name, store.Trusted{Countries: countries, IPs: ips}); err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	s.audit(r, "user.trusted", name, "ok")
	writeJSON(w, http.StatusOK, trustedRequest{Countries: countries, IPs: ips})
}
