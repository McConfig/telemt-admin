// Package userimport разбирает CSV с клиентами и проверяет поля.
package userimport

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MaxRows — максимум клиентов в одном файле.
const MaxRows = 1000

var (
	nameRe   = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
	secretRe = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

var columns = map[string]bool{
	"name": true, "secret": true, "max_unique_ips": true, "max_tcp_conns": true,
	"data_quota_bytes": true, "quota_period": true, "rate_limit_up_bps": true,
	"rate_limit_down_bps": true, "expiration_rfc3339": true, "enabled": true,
}

// Row — проверенные поля создания клиента.
type Row struct {
	Name              string
	Secret            string
	MaxUniqueIPs      *uint64
	MaxTCPConns       *uint64
	DataQuotaBytes    *uint64
	QuotaPeriod       *string
	RateLimitUpBps    *uint64
	RateLimitDownBps  *uint64
	ExpirationRFC3339 *string
	Enabled           *bool
}

// Item — одна строка CSV и ошибка разбора, если она есть.
type Item struct {
	Line int
	Name string
	Row  Row
	Err  string
}

// Parse читает CSV. Ошибка возвращается только для файла целиком:
// отсутствующий или неверный заголовок, битый CSV, превышение лимита строк.
func Parse(src io.Reader) ([]Item, error) {
	raw, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать CSV: %w", err)
	}
	text := strings.TrimPrefix(string(raw), "\ufeff")
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("пустой CSV")
	}
	delim := ','
	firstLine := text
	if i := strings.IndexByte(firstLine, '\n'); i >= 0 {
		firstLine = firstLine[:i]
	}
	if strings.Count(firstLine, ";") > strings.Count(firstLine, ",") {
		delim = ';'
	}
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = delim
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать заголовок CSV: %w", err)
	}
	index := make(map[string]int, len(header))
	for i, name := range header {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			return nil, fmt.Errorf("колонка %d: пустое имя", i+1)
		}
		if !columns[name] {
			return nil, fmt.Errorf("неизвестная колонка %q", name)
		}
		if _, ok := index[name]; ok {
			return nil, fmt.Errorf("колонка %q повторяется", name)
		}
		index[name] = i
	}
	if _, ok := index["name"]; !ok {
		return nil, errors.New("нет обязательной колонки name")
	}

	items := make([]Item, 0, 16)
	seen := make(map[string]bool)
	for {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("ошибка CSV: %w", err)
		}
		if len(items) == MaxRows {
			return nil, fmt.Errorf("не больше %d строк за раз", MaxRows)
		}
		line, _ := r.FieldPos(0)
		item := Item{Line: line}
		value := func(column string) string {
			i, ok := index[column]
			if !ok || i >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[i])
		}
		item.Name = value("name")
		if len(record) > len(header) {
			item.Err = "в строке больше полей, чем в заголовке"
			items = append(items, item)
			continue
		}
		item.Err = parseRow(&item.Row, item.Name, func(column string) string { return value(column) })
		if nameRe.MatchString(item.Name) {
			if item.Err == "" && seen[item.Name] {
				item.Err = "имя повторяется в файле"
			}
			seen[item.Name] = true
		}
		items = append(items, item)
	}
	return items, nil
}

func parseRow(row *Row, name string, value func(string) string) string {
	if !nameRe.MatchString(name) {
		return "имя: латиница, цифры, точка, дефис, подчёркивание, до 64 символов"
	}
	row.Name = name
	row.Secret = value("secret")
	if row.Secret != "" && !secretRe.MatchString(row.Secret) {
		return "секрет: 32 hex-символа в нижнем регистре"
	}
	var errText string
	if row.MaxUniqueIPs, errText = uintField(value, "max_unique_ips", 1000); errText != "" {
		return errText
	}
	if row.MaxTCPConns, errText = uintField(value, "max_tcp_conns", 10000); errText != "" {
		return errText
	}
	if row.DataQuotaBytes, errText = uintField(value, "data_quota_bytes", 1<<50); errText != "" {
		return errText
	}
	if period := value("quota_period"); period != "" {
		switch period {
		case "daily", "weekly", "monthly":
			row.QuotaPeriod = &period
		default:
			return "quota_period: daily, weekly или monthly"
		}
	}
	if row.RateLimitUpBps, errText = uintField(value, "rate_limit_up_bps", 1_000_000_000_000); errText != "" {
		return errText
	}
	if row.RateLimitDownBps, errText = uintField(value, "rate_limit_down_bps", 1_000_000_000_000); errText != "" {
		return errText
	}
	if expiration := value("expiration_rfc3339"); expiration != "" {
		if _, err := time.Parse(time.RFC3339, expiration); err != nil {
			return "expiration_rfc3339: время в формате RFC3339"
		}
		row.ExpirationRFC3339 = &expiration
	}
	if enabled := value("enabled"); enabled != "" {
		switch strings.ToLower(enabled) {
		case "true":
			v := true
			row.Enabled = &v
		case "false":
			v := false
			row.Enabled = &v
		default:
			return "enabled: true или false"
		}
	}
	return ""
}

func uintField(value func(string) string, column string, max uint64) (*uint64, string) {
	raw := value(column)
	if raw == "" {
		return nil, ""
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return nil, column + ": целое неотрицательное число"
	}
	if n > max {
		return nil, fmt.Sprintf("%s: не больше %d", column, max)
	}
	return &n, ""
}
