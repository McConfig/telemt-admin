package telemt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// APIError — ошибка Control API.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telemt API %d: %s", e.Status, e.Message)
}

// Client — клиент Control API telemt.
type Client struct {
	base  string
	token string
	hc    *http.Client
}

// New создаёт клиент.
func New(baseURL, token string, timeout time.Duration) *Client {
	return &Client{
		base:  strings.TrimRight(baseURL, "/"),
		token: token,
		hc:    &http.Client{Timeout: timeout},
	}
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// redact убирает из текста секреты, если API вернул их в сообщении.
func redact(s string) string {
	re := regexp.MustCompile(`(?i)(secret=)[0-9a-f]+|\b[0-9a-f]{32}\b`)
	return re.ReplaceAllString(s, "${1}***")
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", c.token)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("запрос к telemt: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	var env envelope
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &env)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := strings.TrimSpace(http.StatusText(resp.StatusCode))
		if env.Error != nil && env.Error.Message != "" {
			msg = redact(env.Error.Message)
		}
		return &APIError{Status: resp.StatusCode, Message: msg}
	}
	if len(raw) == 0 {
		return nil
	}
	if env.Data == nil {
		return fmt.Errorf("ответ telemt: нет поля data")
	}
	if out != nil {
		if len(env.Data) == 0 {
			return fmt.Errorf("telemt API: пустой data")
		}
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("разбор data: %w", err)
		}
	}
	return nil
}

// Ready проверяет доступность Control API.
func (c *Client) Ready(ctx context.Context) error {
	var out map[string]any
	return c.do(ctx, "GET", "/v1/capabilities", nil, &out)
}
