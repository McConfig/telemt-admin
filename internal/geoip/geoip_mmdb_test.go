package geoip

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
	"go4.org/netipx"
)

// buildTestDB собирает маленькую mmdb в формате, который ожидает резолвер.
func buildTestDB(t *testing.T, name, dbType string, record mmdbtype.Map) string {
	t.Helper()
	tree, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType:            dbType,
		RecordSize:              24,
		IncludeReservedNetworks: true,
	})
	if err != nil {
		t.Fatalf("создание базы %s: %v", name, err)
	}
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	if err := tree.Insert(netipx.PrefixIPNet(prefix), record); err != nil {
		t.Fatalf("запись %s: %v", name, err)
	}
	path := filepath.Join(t.TempDir(), name)
	file, err := os.Create(path) // #nosec G304 -- путь собирает сам тест.
	if err != nil {
		t.Fatalf("файл %s: %v", name, err)
	}
	if _, err := tree.WriteTo(file); err != nil {
		_ = file.Close()
		t.Fatalf("запись %s: %v", name, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("закрытие %s: %v", name, err)
	}
	return path
}

func TestLookupWithDatabases(t *testing.T) {
	city := buildTestDB(t, "city.mmdb", "GeoLite2-City", mmdbtype.Map{
		"country": mmdbtype.Map{
			"iso_code": mmdbtype.String("NL"),
			"names": mmdbtype.Map{
				"en": mmdbtype.String("Netherlands"),
			},
		},
	})
	asn := buildTestDB(t, "asn.mmdb", "GeoLite2-ASN", mmdbtype.Map{
		"autonomous_system_number":       mmdbtype.Uint32(60781),
		"autonomous_system_organization": mmdbtype.String("LeaseWeb"),
	})

	r, err := Open(city, asn)
	if err != nil {
		t.Fatalf("открытие баз: %v", err)
	}
	defer r.Close()
	if !r.Enabled() {
		t.Fatal("резолвер должен быть включён")
	}

	info := r.Lookup("192.0.2.5")
	if info.CountryCode != "NL" || info.CountryName != "Нидерланды" {
		t.Fatalf("страна: %+v", info)
	}
	if info.ASN != 60781 || info.ASNOrg != "LeaseWeb" {
		t.Fatalf("ASN: %+v", info)
	}
	if !info.Foreign() {
		t.Fatalf("адрес должен считаться иностранным: %+v", info)
	}

	// Адрес вне тестовой сети: базы есть, но записи нет.
	if got := r.Lookup("8.8.8.8"); !got.Empty() {
		t.Fatalf("ждали пустые сведения, получили %+v", got)
	}
}

func TestOpenMissingFile(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "нет.mmdb"), ""); err == nil {
		t.Fatal("ждали ошибку для отсутствующей базы")
	}
	if _, err := Open("", ""); err != nil {
		t.Fatalf("пустые пути не должны давать ошибку: %v", err)
	}
}
