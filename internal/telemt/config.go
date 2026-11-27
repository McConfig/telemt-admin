package telemt

import (
	"context"
	"fmt"
)

// ConfigData — редактируемые секции конфигурации и ревизия.
type ConfigData struct {
	Config   map[string]any
	Revision string
}

// SchemaField — описание одного пути конфигурации.
type SchemaField struct {
	Path  string         `json:"path"`
	Value map[string]any `json:"value"`
	API   struct {
		ConfigRead  bool `json:"config_read"`
		ConfigWrite bool `json:"config_write"`
	} `json:"api"`
	Apply struct {
		Watcher string `json:"watcher"`
		Reload  string `json:"reload"`
		Effect  string `json:"effect"`
	} `json:"apply"`
	NonEditableReason string `json:"non_editable_reason"`
}

// ConfigSchema — схема путей конфигурации.
type ConfigSchema struct {
	Version int
	Fields  []SchemaField
}

// ConfigPatchResult — результат применения патча.
type ConfigPatchResult struct {
	Revision               string   `json:"revision"`
	ChangedPaths           []string `json:"changed_paths"`
	RuntimeReloadRequired  bool     `json:"runtime_reload_required"`
	ProcessRestartRequired bool     `json:"process_restart_required"`
	DeferredProcessFields  []string `json:"deferred_process_fields"`
	ReloadID               int64    `json:"-"`
}

// ReloadStatus — состояние перезагрузки.
type ReloadStatus struct {
	ID        int64  `json:"reload_id"`
	State     string `json:"state"`
	Phase     string `json:"phase"`
	ErrorCode string `json:"error_code,omitempty"`
	Warnings  int    `json:"warning_count,omitempty"`
}

// GetConfig читает конфигурацию и ревизию.
func (c *Client) GetConfig(ctx context.Context) (*ConfigData, error) {
	var data map[string]any
	rev, _, err := c.doMeta(ctx, "GET", "/v1/config", nil, &data, nil)
	if err != nil {
		return nil, err
	}
	return &ConfigData{Config: data, Revision: rev}, nil
}

// GetConfigSchema читает схему конфигурации.
func (c *Client) GetConfigSchema(ctx context.Context) (*ConfigSchema, error) {
	var data struct {
		SchemaVersion int           `json:"schema_version"`
		Fields        []SchemaField `json:"fields"`
	}
	_, _, err := c.doMeta(ctx, "GET", "/v1/config/schema", nil, &data, nil)
	if err != nil {
		return nil, err
	}
	return &ConfigSchema{Version: data.SchemaVersion, Fields: data.Fields}, nil
}

// ValidateConfig проверяет кандидат без записи.
func (c *Client) ValidateConfig(ctx context.Context, patch map[string]any) error {
	_, _, err := c.doMeta(ctx, "POST", "/v1/config/validate", patch, nil, nil)
	return err
}

// PatchConfig применяет sparse-патч и при необходимости запускает reload.
func (c *Client) PatchConfig(
	ctx context.Context, patch map[string]any, ifMatch string, reload bool,
) (*ConfigPatchResult, error) {
	headers := map[string]string{}
	if ifMatch != "" {
		headers["If-Match"] = ifMatch
	}
	path := "/v1/config"
	if reload {
		path += "?reload=true"
	}
	var data struct {
		Revision               string   `json:"revision"`
		ChangedPaths           []string `json:"changed_paths"`
		RuntimeReloadRequired  bool     `json:"runtime_reload_required"`
		ProcessRestartRequired bool     `json:"process_restart_required"`
		DeferredProcessFields  []string `json:"deferred_process_fields"`
		Reload                 *struct {
			ReloadID int64 `json:"reload_id"`
		} `json:"reload"`
	}
	rev, _, err := c.doMeta(ctx, "PATCH", path, patch, &data, headers)
	if err != nil {
		return nil, err
	}
	out := &ConfigPatchResult{
		Revision:               data.Revision,
		ChangedPaths:           data.ChangedPaths,
		RuntimeReloadRequired:  data.RuntimeReloadRequired,
		ProcessRestartRequired: data.ProcessRestartRequired,
		DeferredProcessFields:  data.DeferredProcessFields,
	}
	if out.Revision == "" {
		out.Revision = rev
	}
	if data.Reload != nil {
		out.ReloadID = data.Reload.ReloadID
	}
	return out, nil
}

// ReloadStatus читает статус перезагрузки.
func (c *Client) ReloadStatus(ctx context.Context, id int64) (*ReloadStatus, error) {
	var data ReloadStatus
	_, _, err := c.doMeta(ctx, "GET", fmt.Sprintf("/v1/system/reload/%d", id), nil, &data, nil)
	if err != nil {
		return nil, err
	}
	if data.ID == 0 {
		data.ID = id
	}
	return &data, nil
}
