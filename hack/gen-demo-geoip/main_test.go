//go:build demo

package main

import (
	"path/filepath"
	"testing"

	"github.com/McConfig/telemt-admin/internal/geoip"
)

func TestBuildAndLookup(t *testing.T) {
	dir := t.TempDir()
	city := filepath.Join(dir, "city.mmdb")
	asn := filepath.Join(dir, "asn.mmdb")

	if err := buildCity(city); err != nil {
		t.Fatalf("buildCity: %v", err)
	}
	if err := buildASN(asn); err != nil {
		t.Fatalf("buildASN: %v", err)
	}

	r, err := geoip.Open(city, asn)
	if err != nil {
		t.Fatalf("geoip.Open: %v", err)
	}
	defer r.Close()

	cases := []struct {
		ip      string
		country string
		asn     uint32
		org     string
	}{
		{ip: "192.0.2.10", country: "RU", asn: 25513, org: "MGTS"},
		{ip: "198.51.100.7", country: "NL", asn: 60781, org: "LeaseWeb"},
		{ip: "203.0.113.9", country: "DE", asn: 24940, org: "Hetzner"},
	}
	for _, c := range cases {
		info := r.Lookup(c.ip)
		if info.CountryCode != c.country || info.ASN != c.asn || info.ASNOrg != c.org {
			t.Fatalf("%s: получено %+v, ожидалось %s/%d/%s", c.ip, info, c.country, c.asn, c.org)
		}
	}

	if info := r.Lookup("10.0.0.1"); !info.Internal {
		t.Fatalf("приватный адрес должен быть Internal: %+v", info)
	}
}
