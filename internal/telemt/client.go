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
	Code    string
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
	OK       bool            `json:"ok"`
	Revision string          `json:"revision"`
	Data     json.RawMessage `json:"data"`
	Error    *struct {
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
	_, _, err := c.doMeta(ctx, method, path, in, out, nil)
	return err
}

func (c *Client) doMeta(
	ctx context.Context, method, path string, in, out any, headers map[string]string,
) (revision string, status int, err error) {
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return "", 0, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return "", 0, err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", c.token)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return "", 0, transportError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", resp.StatusCode, err
	}
	var env envelope
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &env)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := strings.TrimSpace(http.StatusText(resp.StatusCode))
		upstream := ""
		if env.Error != nil && env.Error.Message != "" {
			msg = redact(env.Error.Message)
			upstream = env.Error.Code
		}
		return "", resp.StatusCode, &APIError{
			Status: resp.StatusCode, Code: statusCode(resp.StatusCode, upstream), Message: msg,
		}
	}
	if len(raw) == 0 {
		return env.Revision, resp.StatusCode, nil
	}
	if env.Data == nil {
		return env.Revision, resp.StatusCode, &APIError{Code: CodeInvalidResponse, Message: "ответ Control API без поля data"}
	}
	if out != nil {
		if len(env.Data) == 0 {
			return env.Revision, resp.StatusCode, &APIError{Code: CodeInvalidResponse, Message: "пустой ответ Control API"}
		}
		if err := json.Unmarshal(env.Data, out); err != nil {
			return env.Revision, resp.StatusCode, &APIError{Code: CodeInvalidResponse, Message: "не удалось разобрать ответ Control API"}
		}
	}
	return env.Revision, resp.StatusCode, nil
}

// Capabilities проверяет доступность и возвращает сведения Control API.
func (c *Client) Capabilities(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, "GET", "/v1/capabilities", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Ready проверяет доступность Control API.
func (c *Client) Ready(ctx context.Context) error {
	_, err := c.Capabilities(ctx)
	return err
}
