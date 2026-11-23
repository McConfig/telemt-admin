package telemt

import (
	"context"
	"errors"
	"fmt"
)

// RenamePartialError — новый пользователь создан, старый удалить не удалось.
type RenamePartialError struct {
	Old string
	New string
	Err error
}

func (e *RenamePartialError) Error() string {
	return fmt.Sprintf("создан %q, но %q удалить не удалось: %v", e.New, e.Old, e.Err)
}

func (e *RenamePartialError) Unwrap() error { return e.Err }

// Rename переименовывает пользователя, сохраняя секрет.
func (c *Client) Rename(ctx context.Context, oldName, newName string) error {
	src, err := c.GetUser(ctx, oldName)
	if err != nil {
		return err
	}
	link := src.Link("tls")
	if link == "" {
		return errors.New("у пользователя нет TLS-ссылки, переименование невозможно")
	}
	key, err := KeyFromLink(link)
	if err != nil {
		return err
	}
	in := CreateInput{
		Username:          newName,
		Secret:            key,
		MaxTCPConns:       src.MaxTCPConns,
		MaxUniqueIPs:      src.MaxUniqueIPs,
		ExpirationRFC3339: src.ExpirationRFC3339,
		DataQuotaBytes:    src.DataQuotaBytes,
		QuotaPeriod:       src.QuotaPeriod,
		RateLimitUpBps:    src.RateLimitUpBps,
		RateLimitDownBps:  src.RateLimitDownBps,
	}
	if _, err := c.CreateUser(ctx, in); err != nil {
		return fmt.Errorf("создать %q: %w", newName, err)
	}
	dst, err := c.GetUser(ctx, newName)
	if err != nil {
		_ = c.DeleteUser(ctx, newName)
		return fmt.Errorf("проверить %q: %w", newName, err)
	}
	dstLink := dst.Link("tls")
	if dstLink == "" {
		_ = c.DeleteUser(ctx, newName)
		return fmt.Errorf("у %q нет TLS-ссылки после создания", newName)
	}
	dstKey, err := KeyFromLink(dstLink)
	if err != nil || dstKey != key {
		_ = c.DeleteUser(ctx, newName)
		return errors.New("ключ нового пользователя не совпал со старым")
	}
	if err := c.DeleteUser(ctx, oldName); err != nil {
		return &RenamePartialError{Old: oldName, New: newName, Err: err}
	}
	return nil
}
