package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	h, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("формат хэша: %q", h)
	}
	ok, err := Verify(h, "correct horse battery staple")
	if err != nil || !ok {
		t.Fatalf("Verify верный пароль: %v, %v", ok, err)
	}
	ok, err = Verify(h, "wrong password 123")
	if err != nil || ok {
		t.Fatalf("Verify неверный пароль: %v, %v", ok, err)
	}
	if _, err := Verify("не-хэш", "x"); err == nil {
		t.Fatal("ожидал ошибку формата")
	}
}

func TestCheckPolicy(t *testing.T) {
	if err := CheckPolicy("короткий"); err == nil {
		t.Fatal("короткий пароль должен быть отклонён")
	}
	if err := CheckPolicy("длинный пароль 123"); err != nil {
		t.Fatalf("нормальный пароль отклонён: %v", err)
	}
}

func TestVerifyErrors(t *testing.T) {
	for _, hash := range []string{"", "plain", "$argon2id$v=19$m=65536,t=3,p=2$!!!$!!!"} {
		if ok, err := Verify(hash, "password"); ok || err == nil {
			t.Errorf("%q: ok=%v err=%v", hash, ok, err)
		}
	}
	if ok, err := Verify("$argon2id$v=19$m=1,t=1,p=1$bad$bad", "password"); ok || err != nil {
		t.Errorf("неверный пароль: ok=%v err=%v", ok, err)
	}
	if err := CheckPolicy("short"); err == nil {
		t.Error("короткий пароль прошёл политику")
	}
	if err := CheckPolicy("long password 123"); err != nil {
		t.Errorf("длинный пароль: %v", err)
	}
}
