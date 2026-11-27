package configx

import "time"

// Plan — результат предпросмотра импорта.
type Plan struct {
	ID                    string                    `json:"id"`
	Owner                 string                    `json:"owner"`
	Session               string                    `json:"session"`
	FileName              string                    `json:"fileName"`
	FileHash              string                    `json:"fileHash"`
	CreatedAt             time.Time                 `json:"createdAt"`
	ExpiresAt             time.Time                 `json:"expiresAt"`
	Bundle                *Bundle                   `json:"bundle"`
	TargetSettingsVersion int64                     `json:"targetSettingsVersion"`
	TargetProxyRevision   string                    `json:"targetProxyRevision"`
	Blocks                []BlockPlan               `json:"blocks"`
	Diff                  map[string][]DiffEntry    `json:"diff"`
	Conflicts             map[string][]ConflictItem `json:"conflicts"`
	Deletions             map[string][]string       `json:"deletions"`
}

// BlockPlan — сводка по блоку.
type BlockPlan struct {
	ID       Block    `json:"id"`
	Title    string   `json:"title"`
	Changes  int      `json:"changes"`
	Warnings []string `json:"warnings"`
}

// DiffEntry — изменение одного пути.
type DiffEntry struct {
	Path   string `json:"path"`
	Before any    `json:"before"`
	After  any    `json:"after"`
	Secret bool   `json:"secret,omitempty"`
}

// ConflictItem — конфликт по имени.
type ConflictItem struct {
	Name    string   `json:"name"`
	Action  string   `json:"action"`
	Changes []string `json:"changes"`
}

// Действия при конфликте.
const (
	ActionKeep      = "keep"
	ActionOverwrite = "overwrite"
)
