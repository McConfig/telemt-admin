import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { Alert, Button, Card, Dialog, Label, Loader, Text, TextInput } from '@gravity-ui/uikit';
import { Table, useTable } from '@gravity-ui/table';
import type { ColumnDef } from '@gravity-ui/table/tanstack';
import { api, ApiError, type Me, type ProxyUser } from '../api';
import { formatBytes } from '../format';

export default function UsersPage({ me }: { me: Me }) {
  const [users, setUsers] = useState<ProxyUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [onlyRisk, setOnlyRisk] = useState(false);
  const [page, setPage] = useState(1);
  const [createOpen, setCreateOpen] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      setUsers(await api.users());
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось загрузить список');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return users.filter((u) => {
      if (q && !u.username.toLowerCase().includes(q)) return false;
      if (onlyRisk && u.risk === 0) return false;
      return true;
    });
  }, [users, query, onlyRisk]);
  useEffect(() => {
    setPage(1);
  }, [query, onlyRisk]);
  const pageSize = 20;
  const pages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const shown = filtered.slice((page - 1) * pageSize, page * pageSize);

  const columns: ColumnDef<ProxyUser>[] = [
    {
      id: 'username',
      header: 'Имя',
      cell: ({ row }) => (
        <Link to={`/users/${encodeURIComponent(row.original.username)}`}>{row.original.username}</Link>
      ),
    },
    {
      id: 'status',
      header: 'Статус',
      cell: ({ row }) => (
        <Label theme={row.original.enabled ? 'success' : 'danger'} size="s">
          {row.original.enabled ? 'включён' : 'заблокирован'}
        </Label>
      ),
    },
    { id: 'ips', header: 'Активные IP', cell: ({ row }) => row.original.activeUniqueIps },
    { id: 'ips24h', header: 'IP за 24ч', cell: ({ row }) => row.original.ips24h },
    {
      id: 'risk',
      header: 'Риск',
      cell: ({ row }) => {
        const risk = row.original.risk;
        if (risk === 0) return <Label theme="success" size="s">нет</Label>;
        return (
          <Label theme={risk === 2 ? 'danger' : 'warning'} size="s">
            {risk === 2 ? 'высокий' : 'внимание'}
          </Label>
        );
      },
    },
    { id: 'conns', header: 'Соединения', cell: ({ row }) => row.original.currentConnections },
    { id: 'traffic', header: 'Трафик', cell: ({ row }) => formatBytes(row.original.totalOctets) },
    {
      id: 'last',
      header: 'Последняя активность',
      cell: ({ row }) =>
        row.original.lastSeen ? new Date(row.original.lastSeen).toLocaleString('ru-RU') : '—',
    },
    {
      id: 'actions',
      header: '',
      cell: ({ row }) => (
        <Link to={`/users/${encodeURIComponent(row.original.username)}`}>Открыть</Link>
      ),
    },
  ];

  const table = useTable({ columns, data: shown });

  return (
    <div className="stack">
      <div className="row">
        <Text variant="header-2">Пользователи telemt</Text>
        <div style={{ flex: 1 }} />
        {me.user.role === 'admin' && (
          <Button view="action" onClick={() => setCreateOpen(true)}>
            Создать пользователя
          </Button>
        )}
        <Button view="outlined" onClick={() => void load()}>
          Обновить
        </Button>
      </div>
      <div className="row">
        <TextInput placeholder="Поиск по имени" value={query} onUpdate={setQuery} />
        <label>
          <input type="checkbox" checked={onlyRisk} onChange={(e) => setOnlyRisk(e.target.checked)} /> только
          подозрительные
        </label>
      </div>
      {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
      <Card view="filled" style={{ padding: 8 }}>
        {loading ? (
          <Loader size="m" />
        ) : (
          <Table table={table} emptyContent="Пользователей нет" />
        )}
      </Card>
      {pages > 1 && (
        <div className="row">
          <Button view="outlined" disabled={page <= 1} onClick={() => setPage(page - 1)}>
            Назад
          </Button>
          <Text>
            {page} из {pages}
          </Text>
          <Button view="outlined" disabled={page >= pages} onClick={() => setPage(page + 1)}>
            Вперёд
          </Button>
        </div>
      )}
      <CreateUserDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={() => {
          setCreateOpen(false);
          void load();
        }}
      />
    </div>
  );
}

function CreateUserDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [name, setName] = useState('');
  const [secret, setSecret] = useState('');
  const [maxIps, setMaxIps] = useState('');
  const [maxConns, setMaxConns] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const submit = async () => {
    setBusy(true);
    setError('');
    try {
      await api.create({
        name: name.trim(),
        secret: secret.trim() || undefined,
        maxUniqueIps: maxIps ? Number(maxIps) : undefined,
        maxTcpConns: maxConns ? Number(maxConns) : undefined,
      });
      setName('');
      setSecret('');
      setMaxIps('');
      setMaxConns('');
      onCreated();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось создать');
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()} maxWidth="s">
      <Dialog.Header caption="Новый пользователь" />
      <Dialog.Body>
        <div className="stack">
          {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
          <TextInput placeholder="Имя" value={name} onUpdate={setName} />
          <TextInput
            placeholder="Секрет (32 hex, пусто — сгенерировать)"
            value={secret}
            onUpdate={setSecret}
          />
          <TextInput placeholder="Максимум уникальных IP" value={maxIps} onUpdate={setMaxIps} />
          <TextInput placeholder="Максимум соединений" value={maxConns} onUpdate={setMaxConns} />
        </div>
      </Dialog.Body>
      <Dialog.Footer
        onClickButtonCancel={onClose}
        onClickButtonApply={() => void submit()}
        textButtonApply="Создать"
        textButtonCancel="Отмена"
        propsButtonApply={{ loading: busy, disabled: !name.trim() }}
      />
    </Dialog>
  );
}
