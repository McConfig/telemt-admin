import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { Alert, Button, Card, Dialog, Label, Loader, Text, TextInput } from '@gravity-ui/uikit';
import { Table, useTable } from '@gravity-ui/table';
import type { ColumnDef } from '@gravity-ui/table/tanstack';
import { api, ApiError, type Me, type ProxyUser, type Summary } from '../api';
import { formatBytes } from '../format';
import { useMedia } from '../useMedia';

export default function UsersPage({ me }: { me: Me }) {
  const [users, setUsers] = useState<ProxyUser[]>([]);
  const [summary, setSummary] = useState<Summary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [onlyRisk, setOnlyRisk] = useState(false);
  const [onlyForeign, setOnlyForeign] = useState(false);
  const [onlyNewCountry, setOnlyNewCountry] = useState(false);
  const [page, setPage] = useState(1);
  const [createOpen, setCreateOpen] = useState(false);
  const isMobile = useMedia('(max-width: 760px)');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [list, total] = await Promise.all([
        api.users(),
        api.summary().catch(() => null),
      ]);
      setUsers(list);
      setSummary(total);
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
      if (onlyForeign && u.foreignIpsCurrent === 0) return false;
      if (onlyNewCountry && u.newCountries24h === 0) return false;
      return true;
    });
  }, [users, query, onlyRisk, onlyForeign, onlyNewCountry]);
  useEffect(() => {
    setPage(1);
  }, [query, onlyRisk, onlyForeign, onlyNewCountry]);
  const pageSize = 20;
  const pages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const shown = useMemo(
    () => filtered.slice((page - 1) * pageSize, page * pageSize),
    [filtered, page],
  );

  const columns = useMemo<ColumnDef<ProxyUser>[]>(() => [
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
    {
      id: 'countries',
      header: 'Страны',
      cell: ({ row }) => {
        const countries = row.original.countries ?? [];
        if (countries.length === 0) return '—';
        return (
          <span title={countries.join(', ')}>
            {countries.slice(0, 2).join(', ')}
            {countries.length > 2 ? ` +${countries.length - 2}` : ''}
          </span>
        );
      },
    },
    {
      id: 'newcountry',
      header: 'Новые страны',
      cell: ({ row }) =>
        row.original.newCountries24h > 0 ? (
          <Label theme="warning" size="s">
            {row.original.newCountries24h}
          </Label>
        ) : (
          '—'
        ),
    },
    {
      id: 'foreign',
      header: 'Иностранные',
      cell: ({ row }) => {
        const n = row.original.foreignIpsCurrent;
        if (!n) return <Label theme="success" size="s">нет</Label>;
        return <Label theme="warning" size="s">{n}</Label>;
      },
    },
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
        <Link className="action-link" to={`/users/${encodeURIComponent(row.original.username)}`}>
          Открыть
        </Link>
      ),
    },
  ], []);

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
      {summary && (
        <>
          <div className="stat-grid">
            <SummaryTile label="Клиентов" value={summary.users} />
            <SummaryTile label="Включено" value={summary.enabled} />
            <SummaryTile
              label="Подозрительных"
              value={summary.riskWarn + summary.riskHigh}
              tone={summary.riskHigh > 0 ? 'danger' : summary.riskWarn > 0 ? 'warning' : undefined}
            />
            <SummaryTile label="Иностранных за сутки" value={summary.foreignCurrent} />
            <SummaryTile
              label="Новых стран"
              value={summary.newCountries}
              tone={summary.newCountries > 0 ? 'warning' : undefined}
            />
            <SummaryTile label="Трафик за сутки" value={formatBytes(summary.trafficToday)} />
          </div>
          <Text className="muted">
            Сборщик:{' '}
            {summary.collector.up
              ? `работает, последний сбор ${summary.collector.lastSuccess ? new Date(summary.collector.lastSuccess).toLocaleTimeString('ru-RU') : '—'}`
              : 'не отвечает'}
            {' · '}
            GeoIP:{' '}
            {summary.geoip.loaded && summary.geoip.built
              ? `базы от ${new Date(summary.geoip.built).toLocaleDateString('ru-RU')}`
              : 'не загружены'}
          </Text>
          {!summary.collector.up && (
            <Alert
              theme="danger"
              message="Сборщик статистики не отвечает: история IP и трафик не пополняются. Проверьте контейнер tgadmin и Control API telemt."
            />
          )}
        </>
      )}
      <div className="row">
        <TextInput placeholder="Поиск по имени" value={query} onUpdate={setQuery} />
        <label>
          <input type="checkbox" checked={onlyRisk} onChange={(e) => setOnlyRisk(e.target.checked)} /> только
          подозрительные
        </label>
        <label>
          <input
            type="checkbox"
            checked={onlyForeign}
            onChange={(e) => setOnlyForeign(e.target.checked)}
          />{' '}
          только с иностранными IP
        </label>
        <label>
          <input
            type="checkbox"
            checked={onlyNewCountry}
            onChange={(e) => setOnlyNewCountry(e.target.checked)}
          />{' '}
          только с новыми странами
        </label>
      </div>
      {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
      <Card view="filled" style={{ padding: 8 }}>
        {loading ? (
          <Loader size="m" />
        ) : isMobile ? (
          <div className="user-cards">
            {shown.length === 0 && <Text className="muted">Пользователей нет</Text>}
            {shown.map((u) => (
              <div className="user-card" key={u.username}>
                <div className="row">
                  <Link to={`/users/${encodeURIComponent(u.username)}`}>{u.username}</Link>
                  <Label theme={u.enabled ? 'success' : 'danger'} size="s">
                    {u.enabled ? 'включён' : 'заблокирован'}
                  </Label>
                  {u.risk > 0 && (
                    <Label theme={u.risk === 2 ? 'danger' : 'warning'} size="s">
                      {u.risk === 2 ? 'высокий риск' : 'внимание'}
                    </Label>
                  )}
                </div>
                <div className="muted">
                  IP: {u.activeUniqueIps} · за 24ч: {u.ips24h} · соединений: {u.currentConnections}
                </div>
                {(u.countries?.length ?? 0) > 0 && (
                  <div className="muted">
                    Страны: {u.countries!.join(', ')}
                    {u.foreignIpsCurrent > 0 ? ` · иностранных IP: ${u.foreignIpsCurrent}` : ''}
                  </div>
                )}
                {u.newCountries24h > 0 && (
                  <div className="muted">Новых стран за сутки: {u.newCountries24h}</div>
                )}
                <div className="muted">Трафик: {formatBytes(u.totalOctets)}</div>
                <div className="muted">
                  Последняя активность:{' '}
                  {u.lastSeen ? new Date(u.lastSeen).toLocaleString('ru-RU') : '—'}
                </div>
                <Link className="action-link" to={`/users/${encodeURIComponent(u.username)}`}>
                  Открыть
                </Link>
              </div>
            ))}
          </div>
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

function SummaryTile({
  label,
  value,
  tone,
}: {
  label: string;
  value: string | number;
  tone?: 'warning' | 'danger';
}) {
  return (
    <div className="stat-tile">
      <span className="stat-label">{label}</span>
      <span className={`stat-value${tone ? ` stat-value-${tone}` : ''}`}>{value}</span>
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
