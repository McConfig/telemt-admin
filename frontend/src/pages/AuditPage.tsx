import { useEffect, useState } from 'react';
import { Alert, Button, Card, Loader, Text, TextInput } from '@gravity-ui/uikit';
import { Table, useTable } from '@gravity-ui/table';
import type { ColumnDef } from '@gravity-ui/table/tanstack';
import { api, ApiError, type AuditItem } from '../api';
import { formatTime } from '../format';
import { useTableSort } from '../tableSort';
import { useMedia } from '../useMedia';

const actions = [
  '',
  'auth.login',
  'auth.logout',
  'user.create',
  'user.limits',
  'user.rotate',
  'user.enable',
  'user.disable',
  'user.rename',
  'user.delete',
  'user.trusted',
  'user.note',
  'user.history.clear',
  'admin.create',
  'admin.update',
  'admin.delete',
  'admin.sessions',
];

const pageSize = 50;

export default function AuditPage() {
  const [items, setItems] = useState<AuditItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [q, setQ] = useState('');
  const [action, setAction] = useState('');
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [page, setPage] = useState(0);
  const isMobile = useMedia('(max-width: 760px)');
  const { sorting, sortHeader } = useTableSort([{ id: 'at', desc: true }]);
  const sortParam = sorting.map((item) => `${item.desc ? '-' : ''}${item.id}`).join(',');

  useEffect(() => {
    setPage(0);
  }, [q, action, from, to, sortParam]);

  useEffect(() => {
    setLoading(true);
    setError('');
    api
      .audit({ q, action, from, to, sort: sortParam, limit: pageSize, offset: page * pageSize })
      .then(setItems)
      .catch((err) => setError(err instanceof ApiError ? err.message : 'не удалось загрузить аудит'))
      .finally(() => setLoading(false));
  }, [q, action, from, to, sortParam, page]);

  const columns: ColumnDef<AuditItem>[] = [
    { id: 'at', header: () => sortHeader('at', 'Время'), cell: ({ row }) => formatTime(row.original.at) },
    { id: 'actor', header: () => sortHeader('actor', 'Кто'), cell: ({ row }) => row.original.actor },
    { id: 'action', header: () => sortHeader('action', 'Действие'), cell: ({ row }) => row.original.action },
    { id: 'target', header: () => sortHeader('target', 'Цель'), cell: ({ row }) => row.original.target },
    { id: 'result', header: () => sortHeader('result', 'Результат'), cell: ({ row }) => row.original.result },
    { id: 'ip', header: () => sortHeader('ip', 'IP'), cell: ({ row }) => row.original.ip },
  ];

  const table = useTable({ columns, data: items });

  return (
    <div className="stack">
      <div className="row">
        <Text variant="header-2">Аудит</Text>
        <div style={{ flex: 1 }} />
        <a href={api.auditCSVUrl()} download="audit.csv">
          Экспорт CSV
        </a>
      </div>
      {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
      <div className="row">
        <TextInput placeholder="Поиск по имени, цели, IP" value={q} onUpdate={setQ} />
        <select
          className="select"
          value={action}
          onChange={(event) => setAction(event.target.value)}
        >
          {actions.map((item) => (
            <option value={item} key={item || 'all'}>
              {item || 'все действия'}
            </option>
          ))}
        </select>
        <label className="field field-inline">
          <span>с</span>
          <input type="date" value={from} onChange={(event) => setFrom(event.target.value)} />
        </label>
        <label className="field field-inline">
          <span>по</span>
          <input type="date" value={to} onChange={(event) => setTo(event.target.value)} />
        </label>
      </div>
      <Card view="filled" style={{ padding: 8 }}>
        {loading ? (
          <Loader size="m" />
        ) : isMobile ? (
          <div className="user-cards">
            {items.length === 0 && <Text className="muted">Событий нет</Text>}
            {items.map((e) => (
              <div className="user-card" key={`${e.at}-${e.action}-${e.target}`}>
                <div className="row">
                  <Text>{e.action}</Text>
                  <span className="muted">{formatTime(e.at)}</span>
                </div>
                <div className="muted">
                  {e.actor} → {e.target || '—'} · {e.result}
                </div>
                <div className="muted">{e.ip}</div>
              </div>
            ))}
          </div>
        ) : (
          <Table table={table} emptyContent="Событий нет" />
        )}
      </Card>
      <div className="row">
        <Button view="outlined" disabled={page === 0} onClick={() => setPage(page - 1)}>
          Назад
        </Button>
        <Text className="muted">страница {page + 1}</Text>
        <Button
          view="outlined"
          disabled={items.length < pageSize}
          onClick={() => setPage(page + 1)}
        >
          Вперёд
        </Button>
      </div>
    </div>
  );
}
