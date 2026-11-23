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
