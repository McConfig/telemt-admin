package auth

import "testing"

func TestNewServiceTokenRoundTrip(t *testing.T) {
	id, token, err := NewServiceToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 16 {
		t.Fatalf("id: %q", id)
	}
	gotID, secret, ok := ParseServiceToken(token)
	if !ok || gotID != id || secret == "" {
		t.Fatalf("разбор: %q %q %v", gotID, secret, ok)
	}
	if !VerifyServiceSecret(secret, HashServiceSecret(secret)) {
		t.Fatal("хеш не совпал")
	}
	if VerifyServiceSecret(secret+"x", HashServiceSecret(secret)) {
		t.Fatal("чужой секрет прошёл")
	}
}

func TestParseServiceTokenRejectsBadFormat(t *testing.T) {
	for _, raw := range []string{
		"", "tgadm_x_y", "tgadm_0123456789abcdef_short",
		"Bearer tgadm_0123456789abcdef_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	} {
		if _, _, ok := ParseServiceToken(raw); ok {
			t.Fatalf("принят %q", raw)
		}
	}
}
