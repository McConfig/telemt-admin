import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { Alert, Button, Card, Checkbox, Dialog, Label, Loader, Select, Text, TextInput } from '@gravity-ui/uikit';
import { Table, useTable } from '@gravity-ui/table';
import type { VisibilityState } from '@tanstack/react-table';
import { settingsColumn, usePersistentColumnVisibility } from '../components/ColumnSettings';
import type { ColumnDef } from '@gravity-ui/table/tanstack';
import { api, ApiError, type Me, type ProxyUser, type Summary } from '../api';
import { formatBytes } from '../format';
import { useTableSort } from '../tableSort';
import { useMedia } from '../useMedia';
import ImportDialog from './ImportDialog';

const COLUMN_LABELS: Record<string, string> = {
  username: 'Имя',
  status: 'Статус',
  ips: 'Активные IP',
  countries: 'Страны',
  newcountry: 'Новые страны',
  foreign: 'Иностранные',
  ips24h: 'IP за 24ч',
  risk: 'Риск',
  conns: 'Соединения',
  traffic: 'Трафик',
  last: 'Последняя активность',
};

const DEFAULT_VISIBILITY: VisibilityState = {
  newcountry: false,
  foreign: false,
  ips24h: false,
};

export default function UsersPage({ me }: { me: Me }) {
  const [users, setUsers] = useState<ProxyUser[]>([]);
  const [summary, setSummary] = useState<Summary | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [bulkBusy, setBulkBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [onlyRisk, setOnlyRisk] = useState(false);
  const [onlyForeign, setOnlyForeign] = useState(false);
  const [onlyNewCountry, setOnlyNewCountry] = useState(false);
  const [page, setPage] = useState(1);
  const [createOpen, setCreateOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [resetOpen, setResetOpen] = useState(false);
  const [resetNames, setResetNames] = useState<string[] | null>(null);
  const [resetBusy, setResetBusy] = useState(false);
  const [notice, setNotice] = useState('');
  const [columnVisibility, setColumnVisibility] = usePersistentColumnVisibility(
    me.user.name,
    'users',
    DEFAULT_VISIBILITY,
    `tgadmin-columns:${me.user.name}`,
  );
  const isMobile = useMedia('(max-width: 760px)');
  const { sorting, setSorting, sortHeader, sortRows } = useTableSort();

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
    const list = users.filter((u) => {
      if (q) {
        const haystack = [
          u.username,
          ...(u.countries ?? []),
          ...(u.countryCodes ?? []),
          ...(u.operators ?? []),
        ]
          .join(' ')
          .toLowerCase();
        if (!haystack.includes(q)) return false;
      }
      if (onlyRisk && u.risk === 0) return false;
      if (onlyForeign && u.foreignIpsCurrent === 0) return false;
      if (onlyNewCountry && u.newCountries24h === 0) return false;
      return true;
    });
    return sortRows(list, compareUsers);
  }, [users, query, onlyRisk, onlyForeign, onlyNewCountry, sorting]);
  useEffect(() => {
    setPage(1);
  }, [query, onlyRisk, onlyForeign, onlyNewCountry, sorting]);
  const pageSize = 20;
  const pages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const shown = useMemo(
    () => filtered.slice((page - 1) * pageSize, page * pageSize),
    [filtered, page],
  );

  const isAdmin = me.user.role === 'admin';
  const shownNames = shown.map((u) => u.username);
  const allShownSelected =
    shownNames.length > 0 && shownNames.every((name) => selected.has(name));

  const toggleOne = (name: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  };
  const toggleAll = () => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (allShownSelected) shownNames.forEach((name) => next.delete(name));
      else shownNames.forEach((name) => next.add(name));
      return next;
    });
  };
  const bulk = async (action: (name: string) => Promise<unknown>) => {
    setBulkBusy(true);
    setError('');
    try {
      for (const name of selected) await action(name);
      setSelected(new Set());
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'массовое действие не выполнено');
    } finally {
      setBulkBusy(false);
    }
  };
  const exportLinks = () => {
    const rows = users
      .filter((u) => selected.has(u.username))
      .map((u) => `${u.username},"${(u.link ?? '').replace(/"/g, '""')}"`);
    const blob = new Blob([`username,link\n${rows.join('\n')}\n`], {
      type: 'text/csv;charset=utf-8',
    });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = 'telemt-links.csv';
    link.click();
    URL.revokeObjectURL(url);
  };

  const columns = useMemo<ColumnDef<ProxyUser>[]>(() => [
    {
      id: 'select',
      header: () => (
        <Checkbox
          controlProps={{ 'aria-label': 'Выбрать всех' }}
          checked={allShownSelected}
          onUpdate={toggleAll}
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          controlProps={{ 'aria-label': `Выбрать ${row.original.username}` }}
          checked={selected.has(row.original.username)}
          onUpdate={() => toggleOne(row.original.username)}
        />
      ),
    },
    {
      id: 'username',
      header: () => sortHeader('username', 'Имя'),
      cell: ({ row }) => (
        <Link to={`/users/${encodeURIComponent(row.original.username)}`}>{row.original.username}</Link>
      ),
    },
    {
      id: 'status',
      header: () => sortHeader('status', 'Статус'),
      cell: ({ row }) => (
        <Label theme={row.original.enabled ? 'success' : 'danger'} size="s">
          {row.original.enabled ? 'включён' : 'заблокирован'}
        </Label>
      ),
    },
    { id: 'ips', header: () => sortHeader('ips', 'Активные IP'), cell: ({ row }) => row.original.activeUniqueIps },
    {
      id: 'countries',
      header: () => sortHeader('countries', 'Страны'),
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
      header: () => sortHeader('newcountry', 'Новые страны'),
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
      header: () => sortHeader('foreign', 'Иностранные'),
      cell: ({ row }) => {
        const n = row.original.foreignIpsCurrent;
        if (!n) return <Label theme="success" size="s">нет</Label>;
        return <Label theme="warning" size="s">{n}</Label>;
      },
    },
    { id: 'ips24h', header: () => sortHeader('ips24h', 'IP за 24ч'), cell: ({ row }) => row.original.ips24h },
    {
      id: 'risk',
      header: () => sortHeader('risk', 'Риск'),
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
    { id: 'conns', header: () => sortHeader('conns', 'Соединения'), cell: ({ row }) => row.original.currentConnections },
    { id: 'traffic', header: () => sortHeader('traffic', 'Трафик'), cell: ({ row }) => formatBytes(row.original.totalOctets) },
    {
      id: 'last',
      header: () => sortHeader('last', 'Последняя активность'),
      cell: ({ row }) =>
        row.original.lastSeen ? new Date(row.original.lastSeen).toLocaleString('ru-RU') : '—',
    },
    settingsColumn<ProxyUser>(COLUMN_LABELS),
  ], [allShownSelected, selected, shownNames.join(','), sorting]);

  const table = useTable({
    columns,
    data: shown,
    enableHiding: true,
    state: { columnVisibility },
    onColumnVisibilityChange: setColumnVisibility,
    initialState: { columnVisibility: DEFAULT_VISIBILITY },
  });

  return (
    <div className="stack">
      <div className="row">
        <Text variant="header-2">Пользователи telemt</Text>
        <div style={{ flex: 1 }} />
        {me.user.role === 'admin' && (
          <>
            <Button view="outlined" onClick={() => setImportOpen(true)}>
              Импорт CSV
            </Button>
            <Button
              view="outlined"
              onClick={() => {
                setResetNames(null);
                setResetOpen(true);
              }}
            >
              Сбросить все квоты
            </Button>
            <Button view="action" onClick={() => setCreateOpen(true)}>
              Создать пользователя
            </Button>
          </>
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
        <TextInput
          placeholder="Поиск по имени, стране, оператору"
          value={query}
          onUpdate={setQuery}
        />
        {isMobile && (
          <Select
            aria-label="Сортировка"
            value={sorting[0]?.id ? [sorting[0].id] : []}
            options={[
              { value: '', content: 'без сортировки' },
              { value: 'username', content: 'по имени' },
              { value: 'ips', content: 'по активным IP' },
              { value: 'traffic', content: 'по трафику' },
              { value: 'risk', content: 'по риску' },
              { value: 'last', content: 'по активности' },
            ]}
            onUpdate={([value]) => setSorting(value ? [{ id: value, desc: false }] : [])}
            placeholder="без сортировки"
            width={180}
          />
        )}
        <Checkbox
          controlProps={{ 'aria-label': 'только подозрительные' }}
          checked={onlyRisk}
          onUpdate={setOnlyRisk}
          content="только подозрительные"
        />
        <Checkbox
          controlProps={{ 'aria-label': 'только с иностранными IP' }}
          checked={onlyForeign}
          onUpdate={setOnlyForeign}
          content="только с иностранными IP"
        />
        <Checkbox
          controlProps={{ 'aria-label': 'только с новыми странами' }}
          checked={onlyNewCountry}
          onUpdate={setOnlyNewCountry}
          content="только с новыми странами"
        />
      </div>
      {selected.size > 0 && (
        <div className="row bulk-bar">
          <Text>Выбрано: {selected.size}</Text>
          <Button
            view="outlined-danger"
            loading={bulkBusy}
            onClick={() => void bulk((name) => api.disable(name))}
          >
            Отключить
          </Button>
          <Button
            view="outlined"
            loading={bulkBusy}
            onClick={() => void bulk((name) => api.enable(name))}
          >
            Включить
          </Button>
          {isAdmin && (
            <Button
              view="outlined"
              onClick={() => {
                setResetNames([...selected]);
                setResetOpen(true);
              }}
            >
              Сбросить квоты
            </Button>
          )}
          {isAdmin && (
            <Button view="outlined" onClick={exportLinks}>
              Ссылки CSV
            </Button>
          )}
          <Button view="flat" onClick={() => setSelected(new Set())}>
            Снять выбор
          </Button>
        </div>
      )}
      {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
      {notice && <Alert theme="success" message={notice} onClose={() => setNotice('')} />}
      <Card view="filled" style={{ padding: 8 }}>
        {loading ? (
          <Loader size="m" />
        ) : isMobile ? (
          <div className="user-cards">
            {shown.length === 0 && <Text className="muted">Пользователей нет</Text>}
            {shown.map((u) => (
              <div className="user-card" key={u.username}>
                <div className="row">
                  <Checkbox
                    controlProps={{ 'aria-label': `Выбрать ${u.username}` }}
                    checked={selected.has(u.username)}
                    onUpdate={() => toggleOne(u.username)}
                  />
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
          <div className="table-scroll">
            <Table table={table} emptyContent="Пользователей нет" />
          </div>
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
      <Dialog open={resetOpen} onOpenChange={(next) => !next && setResetOpen(false)} maxWidth="s">
        <Dialog.Header caption={resetNames ? 'Сбросить квоты выбранным' : 'Сбросить все квоты'} />
        <Dialog.Body>
          <div className="stack">
            <Text>
              {resetNames
                ? `Будет сброшен расход квоты у ${resetNames.length} клиентов.`
                : 'Будет сброшен расход квоты у всех клиентов.'}
            </Text>
            <Text className="muted">
              Лимиты, сроки, ссылки, история IP и накопленный трафик не изменятся.
            </Text>
          </div>
        </Dialog.Body>
        <Dialog.Footer
          onClickButtonCancel={() => setResetOpen(false)}
          onClickButtonApply={() => {
            setResetBusy(true);
            setError('');
            api
              .bulkResetQuota(resetNames ?? undefined)
              .then((result) => {
                setResetOpen(false);
                setSelected(new Set());
                const parts = [`Сброшено квот: ${result.reset} из ${result.total}`];
                if (result.refused > 0) parts.push(`отказано: ${result.refused}`);
                if (result.unknown > 0) parts.push(`неизвестно: ${result.unknown}`);
                if (result.notSent > 0) parts.push(`не отправлено: ${result.notSent}`);
                setNotice(parts.join(', '));
                void load();
              })
              .catch((err) =>
                setError(err instanceof ApiError ? err.message : 'не удалось сбросить квоты'),
              )
              .finally(() => setResetBusy(false));
          }}
          textButtonApply="Сбросить"
          textButtonCancel="Отмена"
          propsButtonApply={{ loading: resetBusy, view: 'outlined-danger' }}
        />
      </Dialog>
      <ImportDialog
        open={importOpen}
        onClose={() => setImportOpen(false)}
        onImported={() => void load()}
      />
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

function compareUsers(a: ProxyUser, b: ProxyUser, id: string): number {
  switch (id) {
    case 'status':
      if (a.enabled === b.enabled) return 0;
      return a.enabled ? -1 : 1;
    case 'ips':
      return a.activeUniqueIps - b.activeUniqueIps;
    case 'ips24h':
      return a.ips24h - b.ips24h;
    case 'risk':
      return a.risk - b.risk;
    case 'conns':
      return a.currentConnections - b.currentConnections;
    case 'traffic':
      return a.totalOctets - b.totalOctets;
    case 'foreign':
      return a.foreignIpsCurrent - b.foreignIpsCurrent;
    case 'newcountry':
      return a.newCountries24h - b.newCountries24h;
    case 'countries':
      return (a.countries ?? []).join(', ').localeCompare((b.countries ?? []).join(', '));
    case 'last':
      return (a.lastSeen ?? '').localeCompare(b.lastSeen ?? '');
    default:
      return a.username.localeCompare(b.username);
  }
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

