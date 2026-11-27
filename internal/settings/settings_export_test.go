package settings

import (
	"context"
	"encoding/json"
	"testing"
)

func TestExportValuesIncludesActiveAndSkipsReadonly(t *testing.T) {
	svc, _ := testService(t)
	fields := svc.ExportValues(context.Background())
	if len(fields) == 0 {
		t.Fatal("пустой экспорт")
	}
	var interval bool
	for _, f := range fields {
		if f.Key == "collect.interval" {
			interval = f.Value == "1m0s"
		}
		if f.Key == "network.allowed_cidrs" {
			t.Fatal("readonly попал в экспорт")
		}
	}
	if !interval {
		t.Fatal("длительность должна быть строкой")
	}
}

func TestValidateDoesNotWrite(t *testing.T) {
	svc, _ := testService(t)
	before := svc.Version(context.Background())
	if _, err := svc.Validate(context.Background(), map[string]json.RawMessage{
		"risk.warn_ips": json.RawMessage(`4`),
	}, nil); err != nil {
		t.Fatal(err)
	}
	if svc.Version(context.Background()) != before {
		t.Fatal("Validate изменил версию")
	}
	if _, err := svc.Validate(context.Background(), map[string]json.RawMessage{
		"risk.warn_ips": json.RawMessage(`-1`),
	}, nil); err == nil {
		t.Fatal("ожидалась ошибка минимума")
	}
}

func TestActiveKeys(t *testing.T) {
	svc, _ := testService(t)
	keys := svc.ActiveKeys("risk")
	if len(keys) == 0 {
		t.Fatal("нет ключей раздела risk")
	}
}
