// Package geoip определяет страну и автономную систему по IP-адресу.
//
// Используются офлайн-базы в формате MaxMind DB (mmdb): городская и ASN.
// Подходят бесплатные DB-IP Lite и GeoLite2. Если базы не заданы или файл
// не открывается, резолвер считается выключенным и не мешает работе сервиса.
package geoip

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/oschwald/maxminddb-golang/v2"
)

// Info — сведения об адресе.
type Info struct {
	CountryCode string
	CountryName string
	ASN         uint32
	ASNOrg      string
	Internal    bool
}

// Empty сообщает, что по адресу ничего не известно.
func (i Info) Empty() bool {
	return i.CountryCode == "" && i.ASN == 0 && i.ASNOrg == "" && !i.Internal
}

// Foreign сообщает, что адрес принадлежит другой стране.
func (i Info) Foreign() bool {
	return !i.Internal && i.CountryCode != "" && i.CountryCode != "RU"
}

// Resolver — поиск по загруженным базам.
type Resolver struct {
	city *maxminddb.Reader
	asn  *maxminddb.Reader
}

// Open открывает базы. Пустые пути означают «база не нужна».
func Open(cityPath, asnPath string) (*Resolver, error) {
	r := &Resolver{}
	if cityPath != "" {
		reader, err := maxminddb.Open(cityPath)
		if err != nil {
			return nil, fmt.Errorf("городская база %q: %w", cityPath, err)
		}
		r.city = reader
	}
	if asnPath != "" {
		reader, err := maxminddb.Open(asnPath)
		if err != nil {
			r.Close()
			return nil, fmt.Errorf("база ASN %q: %w", asnPath, err)
		}
		r.asn = reader
	}
	return r, nil
}

// Enabled сообщает, открыта ли хотя бы одна база.
func (r *Resolver) Enabled() bool {
	return r != nil && (r.city != nil || r.asn != nil)
}

// Close освобождает файлы баз.
func (r *Resolver) Close() {
	if r == nil {
		return
	}
	if r.city != nil {
		_ = r.city.Close()
	}
	if r.asn != nil {
		_ = r.asn.Close()
	}
}

type cityRecord struct {
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
}

type asnRecord struct {
	Number uint   `maxminddb:"autonomous_system_number"`
	Org    string `maxminddb:"autonomous_system_organization"`
}

// Lookup возвращает сведения об адресе. Для приватных и служебных адресов
// возвращается Internal = true, базы для них не опрашиваются.
func (r *Resolver) Lookup(ip string) Info {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return Info{}
	}
	if isInternal(addr) {
		return Info{Internal: true}
	}
	if r == nil {
		return Info{}
	}
	var out Info
	if r.city != nil {
		var rec cityRecord
		if res := r.city.Lookup(addr); res.Found() {
			if err := res.Decode(&rec); err == nil {
				out.CountryCode = strings.ToUpper(rec.Country.ISOCode)
				out.CountryName = CountryName(out.CountryCode, rec.Country.Names["en"])
			}
		}
	}
	if r.asn != nil {
		var rec asnRecord
		if res := r.asn.Lookup(addr); res.Found() {
			if err := res.Decode(&rec); err == nil {
				out.ASN = uint32(rec.Number)
				out.ASNOrg = rec.Org
			}
		}
	}
	return out
}

func isInternal(addr netip.Addr) bool {
	return addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified()
}
