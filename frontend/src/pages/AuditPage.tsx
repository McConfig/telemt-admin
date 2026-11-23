import { useEffect, useState } from 'react';
import { Alert, Card, Loader, Text } from '@gravity-ui/uikit';
import { Table, useTable } from '@gravity-ui/table';
import type { ColumnDef } from '@gravity-ui/table/tanstack';
import { api, ApiError, type AuditItem } from '../api';
import { formatTime } from '../format';
import { useMedia } from '../useMedia';

export default function AuditPage() {
  const [items, setItems] = useState<AuditItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const isMobile = useMedia('(max-width: 760px)');

  useEffect(() => {
    api
      .audit()
      .then(setItems)
      .catch((err) => setError(err instanceof ApiError ? err.message : 'не удалось загрузить аудит'))
      .finally(() => setLoading(false));
  }, []);

  const columns: ColumnDef<AuditItem>[] = [
    { id: 'at', header: 'Время', cell: ({ row }) => formatTime(row.original.at) },
    { id: 'actor', header: 'Кто', cell: ({ row }) => row.original.actor },
    { id: 'action', header: 'Действие', cell: ({ row }) => row.original.action },
    { id: 'target', header: 'Цель', cell: ({ row }) => row.original.target },
    { id: 'result', header: 'Результат', cell: ({ row }) => row.original.result },
    { id: 'ip', header: 'IP', cell: ({ row }) => row.original.ip },
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
    </div>
  );
}
