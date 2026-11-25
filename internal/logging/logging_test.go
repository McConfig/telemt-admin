package logging

import "testing"

func TestSetAndLevel(t *testing.T) {
	for _, name := range []string{"debug", "info", "warn", "error"} {
		if err := Set(name); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if Level() != name {
			t.Fatalf("%s: получено %s", name, Level())
		}
	}
	if err := Set("verbose"); err == nil {
		t.Fatal("ожидалась ошибка для неизвестного уровня")
	}
	if Handler() == nil {
		t.Fatal("Handler вернул nil")
	}
	_ = Set("info")
}
