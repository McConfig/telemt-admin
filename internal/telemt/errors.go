package telemt

import (
	"context"
	"errors"
	"net"
	"strings"
)

// Коды ошибок Control API. Они стабильны и используются интерфейсом и
// диагностикой; текст сообщения может меняться.
const (
	CodeAuthFailed        = "telemt_auth_failed"
	CodeNotFound          = "telemt_not_found"
	CodeConflict          = "telemt_conflict"
	CodeBadRequest        = "telemt_bad_request"
	CodeConfigReadOnly    = "telemt_config_read_only"
	CodeConfigPermission  = "telemt_config_permission_denied"
	CodeConfigStorageFull = "telemt_config_storage_full"
	CodeConfigWriteFailed = "telemt_config_write_failed"
	CodeConfigReadFailed  = "telemt_config_read_failed"
	CodeTimeout           = "telemt_timeout"
	CodeDNSFailed         = "telemt_dns_failed"
	CodeTLSFailed         = "telemt_tls_failed"
	CodeInvalidResponse   = "telemt_invalid_response"
	CodeAPIError          = "telemt_api_error"
	CodeUnavailable       = "telemt_unavailable"
	CodeUnreachable       = "telemt_unreachable"
)

var hints = map[string]string{
	CodeAuthFailed:        "проверьте TELEMT_API_TOKEN и авторизацию Control API",
	CodeNotFound:          "проверьте имя клиента и версию Control API",
	CodeConflict:          "состояние изменилось, обновите данные и повторите",
	CodeBadRequest:        "проверьте отправленные поля и ограничения telemt",
	CodeConfigReadOnly:    "конфигурация telemt смонтирована только для чтения",
	CodeConfigPermission:  "проверьте владельца и права каталога конфигурации telemt",
	CodeConfigStorageFull: "проверьте свободное место на диске telemt",
	CodeConfigWriteFailed: "проверьте каталог, монтирование и журнал telemt",
	CodeConfigReadFailed:  "проверьте основной файл, include-файлы и права",
	CodeTimeout:           "проверьте нагрузку и время ответа Control API",
	CodeDNSFailed:         "проверьте DNS и имя хоста Control API",
	CodeTLSFailed:         "проверьте сертификат и цепочку доверия Control API",
	CodeInvalidResponse:   "ответ Control API повреждён или несовместим по версии",
	CodeAPIError:          "смотрите журнал telemt и номер запроса",
	CodeUnavailable:       "Control API временно недоступен",
	CodeUnreachable:       "проверьте адрес, порт и доступность Control API",
}

// Hint возвращает подсказку по коду ошибки.
func Hint(code string) string { return hints[code] }

func transportError(err error) *APIError {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return &APIError{Code: CodeTimeout, Message: "Control API не ответил вовремя"}
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return &APIError{Code: CodeDNSFailed, Message: "не удалось определить адрес Control API"}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &APIError{Code: CodeTimeout, Message: "Control API не ответил вовремя"}
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "certificate") || strings.Contains(msg, "tls:") || strings.Contains(msg, "x509") {
		return &APIError{Code: CodeTLSFailed, Message: "не удалось проверить сертификат Control API"}
	}
	return &APIError{Code: CodeUnreachable, Message: "нет соединения с Control API"}
}

func statusCode(status int, upstream string) string {
	switch strings.ToLower(upstream) {
	case "config_permission_denied":
		return CodeConfigPermission
	case "config_read_only":
		return CodeConfigReadOnly
	case "config_storage_full":
		return CodeConfigStorageFull
	case "config_write_failed":
		return CodeConfigWriteFailed
	case "config_read_failed":
		return CodeConfigReadFailed
	case "auth_failed", "unauthorized", "forbidden":
		return CodeAuthFailed
	case "not_found":
		return CodeNotFound
	case "conflict":
		return CodeConflict
	case "bad_request", "invalid", "validation_failed":
		return CodeBadRequest
	}
	switch status {
	case 400, 422:
		return CodeBadRequest
	case 401, 403:
		return CodeAuthFailed
	case 404:
		return CodeNotFound
	case 409:
		return CodeConflict
	case 408, 504:
		return CodeTimeout
	case 502, 503:
		return CodeUnavailable
	default:
		if status >= 500 {
			return CodeAPIError
		}
		return CodeAPIError
	}
}
