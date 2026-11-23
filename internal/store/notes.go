package store

import "context"

func noteKey(user string) string { return "ui:note:" + user }

// Note возвращает заметку администратора о пользователе.
func (s *Store) Note(ctx context.Context, user string) (string, error) {
	v, err := s.rdb.Get(ctx, noteKey(user)).Result()
	if err != nil {
		return "", nil
	}
	return v, nil
}

// SetNote сохраняет заметку; пустая строка удаляет её.
func (s *Store) SetNote(ctx context.Context, user, note string) error {
	if note == "" {
		return s.rdb.Del(ctx, noteKey(user)).Err()
	}
	return s.rdb.Set(ctx, noteKey(user), note, 0).Err()
}

// CountSessions возвращает число активных сессий пользователя UI.
func (s *Store) CountSessions(ctx context.Context, user string) (int64, error) {
	return s.rdb.SCard(ctx, sessionsKey(user)).Result()
}
