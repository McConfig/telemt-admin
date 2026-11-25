// Package logging хранит динамический уровень логов.
package logging

import (
	"fmt"
	"log/slog"
	"os"
)

var level slog.LevelVar

// Handler возвращает текстовый обработчик с текущим уровнем.
func Handler() slog.Handler {
	return slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: &level})
}

// Set меняет уровень: debug, info, warn, error.
func Set(name string) error {
	var next slog.Level
	switch name {
	case "debug":
		next = slog.LevelDebug
	case "info":
		next = slog.LevelInfo
	case "warn":
		next = slog.LevelWarn
	case "error":
		next = slog.LevelError
	default:
		return fmt.Errorf("неизвестный уровень логов: %s", name)
	}
	level.Set(next)
	return nil
}

// Level возвращает текущий уровень.
func Level() string {
	switch level.Level() {
	case slog.LevelDebug:
		return "debug"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	default:
		return "info"
	}
}
