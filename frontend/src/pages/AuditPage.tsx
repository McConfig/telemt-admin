import { useEffect, useState } from 'react';
import { Alert, Card, Loader, Text } from '@gravity-ui/uikit';
import { Table, useTable } from '@gravity-ui/table';
import type { ColumnDef } from '@gravity-ui/table/tanstack';
import { api, ApiError, type AuditItem } from '../api';
import { formatTime } from '../format';

export default function AuditPage() {
  const [items, setItems] = useState<AuditItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

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
      <Text variant="header-2">Аудит</Text>
      {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
      <Card view="filled" style={{ padding: 8 }}>
        {loading ? <Loader size="m" /> : <Table table={table} emptyContent="Событий нет" />}
      </Card>
    </div>
  );
}
