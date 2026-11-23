import { useEffect, useState } from 'react';
import { Alert, Button, Card, Loader, Text, TextInput } from '@gravity-ui/uikit';
import { Table, useTable } from '@gravity-ui/table';
import type { ColumnDef } from '@gravity-ui/table/tanstack';
import { api, ApiError, type SessionItem } from '../api';
import { formatTime } from '../format';
import { useMedia } from '../useMedia';

export default function AccountPage() {
  const [sessions, setSessions] = useState<SessionItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const isMobile = useMedia('(max-width: 760px)');

  const load = () => {
    setLoading(true);
    api
      .sessions()
      .then(setSessions)
      .catch((err) => setError(err instanceof ApiError ? err.message : 'не удалось загрузить сессии'))
      .finally(() => setLoading(false));
  };

  useEffect(load, []);

  const columns: ColumnDef<SessionItem>[] = [
    { id: 'ua', header: 'Устройство', cell: ({ row }) => row.original.userAgent || '—' },
    { id: 'ip', header: 'IP', cell: ({ row }) => row.original.ip },
    { id: 'at', header: 'Вход', cell: ({ row }) => formatTime(row.original.createdAt) },
    {
      id: 'action',
      header: '',
      cell: ({ row }) =>
        row.original.current ? (
          <Text color="secondary">текущая</Text>
        ) : (
          <Button view="flat" onClick={() => void api.revoke(row.original.id).then(load)}>
            Завершить
          </Button>
        ),
    },
  ];

  const changePassword = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError('');
    setMessage('');
    try {
      await api.changePassword(current, next);
      setCurrent('');
      setNext('');
      setMessage('Пароль изменён, другие сессии завершены');
      load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось сменить пароль');
    } finally {
      setBusy(false);
    }
  };

  const table = useTable({ columns, data: sessions });

  return (
    <div className="stack">
      <Text variant="header-2">Аккаунт</Text>
      {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
      {message && <Alert theme="success" message={message} onClose={() => setMessage('')} />}
      <Card view="filled" style={{ padding: 16 }}>
        <form className="stack" onSubmit={changePassword}>
          <Text variant="header-1">Смена пароля</Text>
          <TextInput placeholder="Текущий пароль" type="password" value={current} onUpdate={setCurrent} />
          <TextInput placeholder="Новый пароль (не короче 12)" type="password" value={next} onUpdate={setNext} />
          <div>
            <Button view="action" type="submit" loading={busy} disabled={!current || next.length < 12}>
              Сменить
            </Button>
          </div>
        </form>
      </Card>
      <Card view="filled" style={{ padding: 8 }}>
        <Text variant="header-1" style={{ padding: 8 }}>
          Сессии
        </Text>
        {loading ? (
          <Loader size="m" />
        ) : isMobile ? (
          <div className="user-cards" style={{ padding: 8 }}>
            {sessions.map((item) => (
              <div className="user-card" key={item.id}>
                <div>{item.userAgent || 'устройство не определено'}</div>
                <div className="muted">
                  {item.ip} · {formatTime(item.createdAt)}
                </div>
                {item.current ? (
                  <Text color="secondary">текущая</Text>
                ) : (
                  <Button view="flat" onClick={() => void api.revoke(item.id).then(load)}>
                    Завершить
                  </Button>
                )}
              </div>
            ))}
            {sessions.length === 0 && <Text className="muted">Других сессий нет</Text>}
          </div>
        ) : (
          <Table table={table} emptyContent="Других сессий нет" />
        )}
      </Card>
    </div>
  );
}
