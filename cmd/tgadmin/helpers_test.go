package main

import (
	"net/netip"
	"net/url"

	"github.com/McConfig/telemt-admin/internal/config"
)

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

func mustCIDRs(raw string) []netip.Prefix {
	p, err := netip.ParsePrefix(raw)
	if err != nil {
		panic(err)
	}
	return []netip.Prefix{p}
}

var _ = config.Config{}
