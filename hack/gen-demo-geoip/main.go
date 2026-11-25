//go:build demo

// Command gen-demo-geoip собирает маленькие mmdb для демо-стенда.
//
// Базы содержат только диапазоны RFC 5737 и нужны, чтобы демо показывало
// страны и операторов без лицензионных GeoLite2/DB-IP. Реальные адреса
// в базы не попадают.
//
// Запуск:
//
//	go run ./hack/gen-demo-geoip
package main

import (
	"flag"
	"fmt"
	"log"
	"net/netip"
	"os"
	"path/filepath"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
	"go4.org/netipx"
)

// buildEpoch — фиксированное время сборки, чтобы файлы не менялись при
// каждом запуске. 2026-11-01T00:00:00Z.
const buildEpoch = 1_793_491_200

type network struct {
	prefix  string
	country string
	name    string
	asn     uint32
	org     string
}

var networks = []network{
	{prefix: "192.0.2.0/24", country: "RU", name: "Russia", asn: 25513, org: "MGTS"},
	{prefix: "198.51.100.0/24", country: "NL", name: "Netherlands", asn: 60781, org: "LeaseWeb"},
	{prefix: "203.0.113.0/24", country: "DE", name: "Germany", asn: 24940, org: "Hetzner"},
}

func main() {
	out := flag.String("out", "deploy/demo/geoip", "каталог для mmdb")
	flag.Parse()

	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatalf("каталог %s: %v", *out, err)
	}
	city := filepath.Join(*out, "demo-city.mmdb")
	asn := filepath.Join(*out, "demo-asn.mmdb")

	if err := buildCity(city); err != nil {
		log.Fatalf("городская база: %v", err)
	}
	if err := buildASN(asn); err != nil {
		log.Fatalf("база ASN: %v", err)
	}
	fmt.Printf("готово: %s, %s\n", city, asn)
}

func buildCity(path string) error {
	tree, err := newTree("GeoLite2-City")
	if err != nil {
		return err
	}
	for _, n := range networks {
		record := mmdbtype.Map{
			"country": mmdbtype.Map{
				"iso_code": mmdbtype.String(n.country),
				"names": mmdbtype.Map{
					"en": mmdbtype.String(n.name),
				},
			},
		}
		if err := insert(tree, n.prefix, record); err != nil {
			return err
		}
	}
	return write(tree, path)
}

func buildASN(path string) error {
	tree, err := newTree("GeoLite2-ASN")
	if err != nil {
		return err
	}
	for _, n := range networks {
		record := mmdbtype.Map{
			"autonomous_system_number":       mmdbtype.Uint32(n.asn),
			"autonomous_system_organization": mmdbtype.String(n.org),
		}
		if err := insert(tree, n.prefix, record); err != nil {
			return err
		}
	}
	return write(tree, path)
}

func newTree(dbType string) (*mmdbwriter.Tree, error) {
	return mmdbwriter.New(mmdbwriter.Options{
		DatabaseType:            dbType,
		RecordSize:              24,
		BuildEpoch:              buildEpoch,
		IncludeReservedNetworks: true,
	})
}

func insert(tree *mmdbwriter.Tree, prefix string, record mmdbtype.Map) error {
	p, err := netip.ParsePrefix(prefix)
	if err != nil {
		return fmt.Errorf("префикс %q: %w", prefix, err)
	}
	if err := tree.Insert(netipx.PrefixIPNet(p), record); err != nil {
		return fmt.Errorf("запись %q: %w", prefix, err)
	}
	return nil
}

func write(tree *mmdbwriter.Tree, path string) error {
	file, err := os.Create(path) // #nosec G304 -- путь задаёт оператор генератора.
	if err != nil {
		return err
	}
	if _, err := tree.WriteTo(file); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
