package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"regexp"
)

const serviceTokenPrefix = "tgadm_"

var serviceTokenRe = regexp.MustCompile(`^tgadm_([0-9a-f]{16})_([A-Za-z0-9_-]{43})$`)

// NewServiceToken создаёт публичный ID и полный секрет сервисного токена.
func NewServiceToken() (string, string, error) {
	idRaw := make([]byte, 8)
	secretRaw := make([]byte, 32)
	if _, err := rand.Read(idRaw); err != nil {
		return "", "", err
	}
	if _, err := rand.Read(secretRaw); err != nil {
		return "", "", err
	}
	id := hex.EncodeToString(idRaw)
	secret := base64.RawURLEncoding.EncodeToString(secretRaw)
	return id, serviceTokenPrefix + id + "_" + secret, nil
}

// ParseServiceToken разбирает полный токен на ID и секрет.
func ParseServiceToken(raw string) (string, string, bool) {
	m := serviceTokenRe.FindStringSubmatch(raw)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// HashServiceSecret возвращает hex SHA-256 секрета.
func HashServiceSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// VerifyServiceSecret сравнивает секрет с сохранённым хешем за постоянное время.
func VerifyServiceSecret(secret, hash string) bool {
	return subtle.ConstantTimeCompare(
		[]byte(HashServiceSecret(secret)), []byte(hash),
	) == 1
}
