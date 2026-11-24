package totp

import (
	"strings"
	"testing"
	"time"
)

func TestCodeRFCVector(t *testing.T) {
	// RFC 6238, SHA-1, T=59: восьмизначный код 94287082, шестизначный 287082.
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	code, err := Code(secret, time.Unix(59, 0))
	if err != nil {
		t.Fatal(err)
	}
	if code != "287082" {
		t.Fatalf("код %q, ждали 287082", code)
	}
}

func TestValidateWindow(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	at := time.Unix(1_700_000_000, 0)
	code, _ := Code(secret, at)
	counter, ok, err := Validate(secret, code, at, 1)
	if err != nil || !ok {
		t.Fatalf("текущий код не принят: %v %v", ok, err)
	}
	if counter != at.Unix()/30 {
		t.Fatalf("шаг %d", counter)
	}

	prev, _ := Code(secret, at.Add(-30*time.Second))
	if _, ok, _ := Validate(secret, prev, at, 1); !ok {
		t.Fatal("код предыдущего шага должен приниматься")
	}
	old, _ := Code(secret, at.Add(-2*time.Minute))
	if _, ok, _ := Validate(secret, old, at, 1); ok {
		t.Fatal("старый код не должен приниматься")
	}
	if _, ok, _ := Validate(secret, "000000", at, 1); ok {
		t.Fatal("неверный код принят")
	}
}

func TestEncryptRoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	secret, _ := GenerateSecret()
	enc, err := Encrypt(key, secret)
	if err != nil {
		t.Fatal(err)
	}
	if enc == secret || strings.Contains(enc, secret) {
		t.Fatal("секрет не зашифрован")
	}
	plain, err := Decrypt(key, enc)
	if err != nil {
		t.Fatal(err)
	}
	if plain != secret {
		t.Fatalf("после расшифровки %q", plain)
	}
	if _, err := Decrypt([]byte("ffffffffffffffffffffffffffffffff"), enc); err == nil {
		t.Fatal("чужим ключом расшифровалось")
	}
}

func TestRecoveryCodes(t *testing.T) {
	codes, err := GenerateRecoveryCodes(8)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, code := range codes {
		if len(code) != 9 || code[4] != '-' {
			t.Fatalf("формат кода %q", code)
		}
		hash := HashRecovery(code)
		if len(hash) != 64 {
			t.Fatalf("хеш %q", hash)
		}
		if HashRecovery(strings.ToLower(strings.ReplaceAll(code, "-", " "))) != hash {
			t.Fatal("нормализация кода не совпала")
		}
		if seen[hash] {
			t.Fatal("повторяющийся код")
		}
		seen[hash] = true
	}
}

func TestProvisioningURL(t *testing.T) {
	got := ProvisioningURL("TeleMT Admin", "alice", "JBSWY3DPEHPK3PXP")
	if !strings.HasPrefix(got, "otpauth://totp/TeleMT%20Admin:alice?") {
		t.Fatalf("ссылка %q", got)
	}
	if !strings.Contains(got, "secret=JBSWY3DPEHPK3PXP") {
		t.Fatalf("нет секрета: %q", got)
	}
}

func TestParseKey(t *testing.T) {
	if key, err := ParseKey(""); err != nil || key != nil {
		t.Fatalf("пустой ключ: %v %v", key, err)
	}
	// 32 байта в base64.
	valid := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	key, err := ParseKey(valid)
	if err != nil || len(key) != 32 {
		t.Fatalf("ключ: %d %v", len(key), err)
	}
	if _, err := ParseKey("abc"); err == nil {
		t.Fatal("короткий ключ принят")
	}
}
