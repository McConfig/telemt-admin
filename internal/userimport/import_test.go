package userimport

import (
	"strings"
	"testing"
)

const fullHeader = "name,secret,max_unique_ips,max_tcp_conns,data_quota_bytes,quota_period,rate_limit_up_bps,rate_limit_down_bps,expiration_rfc3339,enabled\n"

func TestParseFullRow(t *testing.T) {
	csv := "\ufeff" + fullHeader +
		"alice,0123456789abcdef0123456789abcdef,5,10,10737418240,monthly,1000,2000,2026-12-31T23:59:59Z,false\n"
	items, err := Parse(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("строк: %d", len(items))
	}
	it := items[0]
	if it.Err != "" {
		t.Fatalf("ошибка строки: %s", it.Err)
	}
	if it.Line != 2 || it.Name != "alice" {
		t.Fatalf("строка: %+v", it)
	}
	if it.Row.Secret != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("секрет: %q", it.Row.Secret)
	}
	if it.Row.MaxUniqueIPs == nil || *it.Row.MaxUniqueIPs != 5 {
		t.Fatalf("max ips: %+v", it.Row.MaxUniqueIPs)
	}
	if it.Row.MaxTCPConns == nil || *it.Row.MaxTCPConns != 10 {
		t.Fatalf("max conns: %+v", it.Row.MaxTCPConns)
	}
	if it.Row.DataQuotaBytes == nil || *it.Row.DataQuotaBytes != 10737418240 {
		t.Fatalf("квота: %+v", it.Row.DataQuotaBytes)
	}
	if it.Row.QuotaPeriod == nil || *it.Row.QuotaPeriod != "monthly" {
		t.Fatalf("период: %+v", it.Row.QuotaPeriod)
	}
	if it.Row.RateLimitUpBps == nil || *it.Row.RateLimitUpBps != 1000 {
		t.Fatalf("скорость вверх: %+v", it.Row.RateLimitUpBps)
	}
	if it.Row.RateLimitDownBps == nil || *it.Row.RateLimitDownBps != 2000 {
		t.Fatalf("скорость вниз: %+v", it.Row.RateLimitDownBps)
	}
	if it.Row.ExpirationRFC3339 == nil || *it.Row.ExpirationRFC3339 != "2026-12-31T23:59:59Z" {
		t.Fatalf("срок: %+v", it.Row.ExpirationRFC3339)
	}
	if it.Row.Enabled == nil || *it.Row.Enabled {
		t.Fatalf("enabled: %+v", it.Row.Enabled)
	}
}

func TestParseMinimalSemicolonAndEmptyFields(t *testing.T) {
	csv := "name;secret\nbob;\n"
	items, err := Parse(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) != 1 || items[0].Err != "" || items[0].Name != "bob" {
		t.Fatalf("items: %+v", items)
	}
	if items[0].Row.Secret != "" || items[0].Row.MaxUniqueIPs != nil || items[0].Row.Enabled != nil {
		t.Fatalf("пустые поля должны быть nil: %+v", items[0].Row)
	}
}

func TestParseRowErrors(t *testing.T) {
	line := func(name, secret, ips, conns, quota, period, up, down, exp, enabled string) string {
		return strings.Join([]string{name, secret, ips, conns, quota, period, up, down, exp, enabled}, ",") + "\n"
	}
	csv := fullHeader +
		line("bad name", "", "1", "", "", "", "", "", "", "") +
		line("alice", "not-hex", "1", "", "", "", "", "", "", "") +
		line("bob", "", "x", "", "", "", "", "", "", "") +
		line("carol", "", "1", "no", "", "", "", "", "", "") +
		line("dave", "", "", "", "no", "", "", "", "", "") +
		line("erin", "", "", "", "", "yearly", "", "", "", "") +
		line("frank", "", "", "", "", "", "", "", "not-a-date", "") +
		line("grace", "", "", "", "", "", "", "", "", "maybe") +
		line("alice", "", "", "", "", "", "", "", "", "")
	items, err := Parse(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) != 9 {
		t.Fatalf("строк: %d", len(items))
	}
	for i, want := range []string{
		"имя", "секрет", "max_unique_ips", "max_tcp_conns", "data_quota_bytes",
		"quota_period", "expiration_rfc3339", "enabled", "повторяется",
	} {
		if items[i].Err == "" || !strings.Contains(items[i].Err, want) {
			t.Fatalf("строка %d: err=%q, ожидалось %q", i, items[i].Err, want)
		}
	}
}

func TestParseHeaderErrors(t *testing.T) {
	for name, csv := range map[string]string{
		"нет имени":        "secret\nx\n",
		"неизвестная":      "name,unknown\nalice,x\n",
		"дубликат":         "name,name\nalice,alice\n",
		"пустой файл":      "",
		"битые кавычки":    "name\n\"alice\n",
		"только заголовок": "name\n",
	} {
		t.Run(name, func(t *testing.T) {
			items, err := Parse(strings.NewReader(csv))
			if err == nil && name != "только заголовок" {
				t.Fatalf("ожидалась ошибка, items=%+v", items)
			}
			if name == "только заголовок" && (err != nil || len(items) != 0) {
				t.Fatalf("заголовок без строк: items=%+v err=%v", items, err)
			}
		})
	}
}

func TestParseRowLimit(t *testing.T) {
	var b strings.Builder
	b.WriteString("name\n")
	for i := 0; i <= MaxRows; i++ {
		b.WriteString("user")
		b.WriteString(strings.Repeat("x", 1))
		b.WriteString("\n")
	}
	if _, err := Parse(strings.NewReader(b.String())); err == nil {
		t.Fatal("ожидалась ошибка лимита строк")
	}
}

func TestParseCaps(t *testing.T) {
	csv := fullHeader +
		"alice,,1001,10001,1125899906842625,monthly,1000000000001,1000000000001,,\n"
	items, err := Parse(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Err == "" {
		t.Fatal("превышения лимитов должны быть ошибкой")
	}
}
