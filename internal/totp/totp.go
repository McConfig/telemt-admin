// Package totp реализует TOTP по RFC 6238 без внешних зависимостей:
// генерацию секрета, проверку кода, резервные коды и шифрование секрета.
package totp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	period = 30
	digits = 6
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateSecret создаёт секрет из 160 бит и возвращает его в Base32.
func GenerateSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return b32.EncodeToString(buf), nil
}

func decodeSecret(secret string) ([]byte, error) {
	clean := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
	if pad := len(clean) % 8; pad != 0 {
		clean += strings.Repeat("=", 8-pad)
	}
	return base32.StdEncoding.DecodeString(clean)
}

// Code считает шестизначный код для секрета и времени.
func Code(secret string, at time.Time) (string, error) {
	key, err := decodeSecret(secret)
	if err != nil {
		return "", err
	}
	return codeForCounter(key, uint64(at.Unix()/period)), nil
}

func codeForCounter(key []byte, counter uint64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", digits, value%1_000_000)
}

// Validate проверяет код с окном window шагов в обе стороны и возвращает
// использованный шаг. Код сравнивается за постоянное время.
func Validate(secret, code string, at time.Time, window int) (int64, bool, error) {
	key, err := decodeSecret(secret)
	if err != nil {
		return 0, false, err
	}
	code = NormalizeCode(code)
	if len(code) != digits {
		return 0, false, nil
	}
	current := int64(at.Unix() / period)
	for delta := -window; delta <= window; delta++ {
		counter := current + int64(delta)
		if counter < 0 {
			continue
		}
		expected := codeForCounter(key, uint64(counter))
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return counter, true, nil
		}
	}
	return 0, false, nil
}

// NormalizeCode убирает пробелы и дефисы, приводит к верхнему регистру.
func NormalizeCode(code string) string {
	replacer := strings.NewReplacer(" ", "", "-", "", "\t", "")
	return strings.ToUpper(replacer.Replace(strings.TrimSpace(code)))
}

// ProvisioningURL собирает otpauth:// ссылку для приложения-аутентификатора.
func ProvisioningURL(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	query := url.Values{}
	query.Set("secret", secret)
	query.Set("issuer", issuer)
	query.Set("algorithm", "SHA1")
	query.Set("digits", fmt.Sprintf("%d", digits))
	query.Set("period", fmt.Sprintf("%d", period))
	return "otpauth://totp/" + label + "?" + query.Encode()
}

// GenerateRecoveryCodes создаёт n резервных кодов вида ABCD-EFGH.
func GenerateRecoveryCodes(n int) ([]string, error) {
	codes := make([]string, 0, n)
	for i := 0; i < n; i++ {
		buf := make([]byte, 5)
		if _, err := rand.Read(buf); err != nil {
			return nil, err
		}
		raw := b32.EncodeToString(buf)
		codes = append(codes, raw[:4]+"-"+raw[4:])
	}
	return codes, nil
}

// HashRecovery возвращает SHA-256 от нормализованного резервного кода.
// Коды длиной 40 бит, поэтому медленный хеш здесь не нужен.
func HashRecovery(code string) string {
	sum := sha256.Sum256([]byte(NormalizeCode(code)))
	return hex.EncodeToString(sum[:])
}

// Encrypt шифрует секрет AES-GCM ключом 32 байта.
func Encrypt(key []byte, plain string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

// Decrypt расшифровывает секрет, зашифрованный Encrypt.
func Decrypt(key []byte, encoded string) (string, error) {
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("короткий шифротекст")
	}
	nonce, sealed := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// ParseKey разбирает base64-ключ шифрования и требует ровно 32 байта.
func ParseKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		key, err = base64.RawStdEncoding.DecodeString(raw)
	}
	if err != nil {
		return nil, fmt.Errorf("TGADMIN_TOTP_KEY: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("TGADMIN_TOTP_KEY: нужно 32 байта, получено %d", len(key))
	}
	return key, nil
}
