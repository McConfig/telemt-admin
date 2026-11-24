package geoip

import "testing"

func TestCountryName(t *testing.T) {
	cases := []struct {
		code, fallback, want string
	}{
		{"RU", "Russia", "Россия"},
		{"NL", "Netherlands", "Нидерланды"},
		{"AQ", "Antarctica", "Antarctica (AQ)"},
		{"AQ", "", "AQ"},
		{"", "Unknown", "Unknown"},
	}
	for _, c := range cases {
		if got := CountryName(c.code, c.fallback); got != c.want {
			t.Errorf("CountryName(%q, %q) = %q, ждали %q", c.code, c.fallback, got, c.want)
		}
	}
}

func TestLookupInternal(t *testing.T) {
	r := &Resolver{}
	for _, ip := range []string{"10.0.0.9", "127.0.0.1", "192.168.1.5", "172.16.0.4", "fe80::1"} {
		info := r.Lookup(ip)
		if !info.Internal {
			t.Errorf("%s: ждали внутренний адрес, получили %+v", ip, info)
		}
	}
}

func TestLookupDisabled(t *testing.T) {
	r := &Resolver{}
	info := r.Lookup("8.8.8.8")
	if !info.Empty() {
		t.Errorf("без баз ждали пустые сведения, получили %+v", info)
	}
	if r.Enabled() {
		t.Error("пустой резолвер не должен считаться включённым")
	}
	if info := r.Lookup("не адрес"); !info.Empty() {
		t.Errorf("некорректный адрес: ждали пустые сведения, получили %+v", info)
	}
}

func TestForeign(t *testing.T) {
	cases := []struct {
		info Info
		want bool
	}{
		{Info{CountryCode: "NL", CountryName: "Нидерланды"}, true},
		{Info{CountryCode: "RU", CountryName: "Россия"}, false},
		{Info{Internal: true}, false},
		{Info{}, false},
	}
	for _, c := range cases {
		if got := c.info.Foreign(); got != c.want {
			t.Errorf("%+v: Foreign() = %v, ждали %v", c.info, got, c.want)
		}
	}
}
