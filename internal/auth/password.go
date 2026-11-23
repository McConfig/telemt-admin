package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// ErrPolicy — пароль не проходит политику.
var ErrPolicy = errors.New("пароль должен быть не короче 12 символов")

const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 2
	argonKeyLen  = 32
	saltLen      = 16
)

// CheckPolicy проверяет минимальные требования к паролю.
func CheckPolicy(password string) error {
	if utf8.RuneCountInString(password) < 12 {
		return ErrPolicy
	}
	return nil
}

// Hash возвращает Argon2id-хэш в стандартном формате.
func Hash(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify сравнивает пароль с хэшем за постоянное время.
func Verify(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, errors.New("неверный формат хэша")
	}
	var memory uint32
	var timeCost uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		return false, fmt.Errorf("параметры Argon2: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	if len(want) == 0 || len(want) > 1024 {
		return false, errors.New("неверная длина хэша")
	}
	// #nosec G115 -- длина ключа ограничена 1024 байтами выше.
	got := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
