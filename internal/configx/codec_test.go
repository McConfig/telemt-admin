package configx

import "testing"

func TestDecodeRejectsUnknownField(t *testing.T) {
	_, err := Decode([]byte("apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata: {name: x, secrets: none}\nspec: {}\nunknown: 1\n"))
	if err == nil {
		t.Fatal("ожидалась ошибка неизвестного поля")
	}
}

func TestDecodeRejectsDuplicateKeys(t *testing.T) {
	_, err := Decode([]byte("apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata:\n  name: a\n  name: b\n"))
	if err == nil {
		t.Fatal("ожидалась ошибка дубля ключа")
	}
}

func TestDecodeRejectsAliases(t *testing.T) {
	_, err := Decode([]byte("apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata: &m\n  name: a\n  secrets: none\nspec: *m\n"))
	if err == nil {
		t.Fatal("ожидалась ошибка алиаса")
	}
}

func TestDecodeRejectsVersion(t *testing.T) {
	_, err := Decode([]byte("apiVersion: tgadmin.telemt/v2\nkind: InstallationBundle\nmetadata: {name: x, secrets: none}\nspec: {}\n"))
	if err == nil {
		t.Fatal("ожидалась ошибка версии")
	}
}

func TestRoundTrip(t *testing.T) {
	b := &Bundle{
		APIVersion: APIVersion, Kind: Kind,
		Metadata: Metadata{Name: "demo", Secrets: SecretsNone},
		Spec:     Spec{Settings: map[string]map[string]any{"risk": {"warn_ips": 3}}},
	}
	raw, err := Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(raw)
	if err != nil || got.Spec.Settings["risk"]["warn_ips"] != 3 {
		t.Fatalf("%+v %v", got, err)
	}
}
