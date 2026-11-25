//go:build demo

package main

import "testing"

func TestGeoFor(t *testing.T) {
	cases := []struct {
		ip      string
		country string
		org     string
	}{
		{ip: "192.0.2.10", country: "RU", org: "MGTS"},
		{ip: "198.51.100.7", country: "NL", org: "LeaseWeb"},
		{ip: "203.0.113.9", country: "DE", org: "Hetzner"},
		{ip: "10.1.2.3", country: "", org: ""},
		{ip: "не-адрес", country: "", org: ""},
	}
	for _, c := range cases {
		g := geoFor(c.ip)
		if g.Country != c.country || g.ASNOrg != c.org {
			t.Fatalf("%s: получено %+v", c.ip, g)
		}
	}
	if !geoFor("10.1.2.3").Internal {
		t.Fatal("приватный адрес должен быть Internal")
	}
}

func TestIpsForDay(t *testing.T) {
	base := []string{"198.51.100.21", "203.0.113.9"}

	if got := ipsForDay("travel-phone", base, 0); len(got) != 1 || got[0] != "198.51.100.21" {
		t.Fatalf("старый день travel-phone: %v", got)
	}
	if got := ipsForDay("travel-phone", base, 6); len(got) != 1 {
		t.Fatalf("вчера travel-phone: %v", got)
	}
	if got := ipsForDay("travel-phone", base, 7); len(got) != 2 {
		t.Fatalf("сегодня travel-phone: %v", got)
	}
	if got := ipsForDay("router-home", []string{"192.0.2.10"}, 1); len(got) != 1 {
		t.Fatalf("router-home до гостя: %v", got)
	}
	if got := ipsForDay("router-home", []string{"192.0.2.10"}, 4); len(got) != 2 {
		t.Fatalf("router-home с гостем: %v", got)
	}
	if got := ipsForDay("phone-main", base, 0); len(got) != 2 {
		t.Fatalf("phone-main не должен меняться в первый день: %v", got)
	}
}
