// Package webhook доставляет события аудита во внешнюю систему.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/McConfig/telemt-admin/internal/settings"
	"github.com/McConfig/telemt-admin/internal/version"
)

// Sender отправляет вебхуки асинхронно, с повторами.
type Sender struct {
	svc    *settings.Service
	client *http.Client
	queue  chan job
	now    func() time.Time
}

type job struct {
	event   string
	payload map[string]any
}

// New создаёт отправитель.
func New(svc *settings.Service) *Sender {
	return &Sender{
		svc:    svc,
		client: &http.Client{Timeout: 10 * time.Second},
		queue:  make(chan job, 128),
		now:    time.Now,
	}
}

// Run обрабатывает очередь до отмены контекста.
func (s *Sender) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case item := <-s.queue:
			if err := s.sendWithRetry(ctx, item.event, item.payload); err != nil {
				slog.Warn("вебхук не доставлен", "event", item.event, "err", err)
			}
		}
	}
}

// Emit ставит событие в очередь, если оно включено в настройках.
func (s *Sender) Emit(ctx context.Context, event string, payload map[string]any) {
	_, _, events, enabled := s.svc.Webhook(ctx)
	if !enabled || !eventEnabled(events, event) {
		return
	}
	select {
	case s.queue <- job{event: event, payload: payload}:
	default:
		slog.Warn("очередь вебхуков заполнена, событие пропущено", "event", event)
	}
}

// SendNow отправляет событие без очереди и фильтра. Нужен для проверки.
func (s *Sender) SendNow(ctx context.Context, event string, payload map[string]any) (int, error) {
	// Перечитываем настройки, чтобы кнопка проверки видела только что
	// сохранённый URL, не дожидаясь кэша.
	_ = s.svc.Load(ctx)
	return s.send(ctx, event, payload)
}

func eventEnabled(events []string, event string) bool {
	for _, item := range events {
		if item == "*" || item == event {
			return true
		}
	}
	return false
}

func (s *Sender) sendWithRetry(ctx context.Context, event string, payload map[string]any) error {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			delay := time.Second
			switch attempt {
			case 2:
				delay = 5 * time.Second
			case 3:
				delay = 15 * time.Second
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		if _, err := s.send(ctx, event, payload); err == nil {
			return nil
		} else {
			last = err
		}
	}
	return last
}

func (s *Sender) send(ctx context.Context, event string, payload map[string]any) (int, error) {
	url, secret, _, enabled := s.svc.Webhook(ctx)
	if !enabled {
		return 0, errors.New("вебхук не настроен")
	}
	body, err := json.Marshal(map[string]any{
		"event":  event,
		"sentAt": s.now().UTC().Format(time.RFC3339),
		"data":   payload,
	})
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "telemt-admin/"+strings.TrimPrefix(version.Version, "v"))
	req.Header.Set("X-TeleMT-Event", event)
	req.Header.Set("X-TeleMT-Delivery", deliveryID())
	timestamp := strconv.FormatInt(s.now().UTC().Unix(), 10)
	req.Header.Set("X-TeleMT-Timestamp", timestamp)
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(timestamp + "." + string(body)))
		req.Header.Set("X-TeleMT-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resp.StatusCode, fmt.Errorf("вебхук ответил %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

func deliveryID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(buf[:])
}
