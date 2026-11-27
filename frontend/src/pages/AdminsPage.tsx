import { useCallback, useEffect, useMemo, useState } from 'react';
import { Alert, Button, Card, Dialog, Label, Loader, Select, Text, TextInput } from '@gravity-ui/uikit';
import { Table, useTable } from '@gravity-ui/table';
import type { ColumnDef } from '@gravity-ui/table/tanstack';
import { ArrowRightFromSquare, Ban, Key, Lock, Play, TrashBin } from '@gravity-ui/icons';
import { api, ApiError, type AdminUser, type Me } from '../api';
import { formatTime } from '../format';
import { useTableSort } from '../tableSort';
import { useMedia } from '../useMedia';
import { settingsColumn, usePersistentColumnVisibility } from '../components/ColumnSettings';

const COLUMN_LABELS: Record<string, string> = {
  name: 'Имя',
  role: 'Роль',
  state: 'Статус',
  totp: '2FA',
  sessions: 'Сессии',
  created: 'Создан',
};

export default function AdminsPage({ me }: { me: Me }) {
  const [items, setItems] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [generated, setGenerated] = useState('');
  const [resetFor, setResetFor] = useState<string | null>(null);
  const isMobile = useMedia('(max-width: 760px)');

  const load = useCallback(() => {
    setLoading(true);
    api
      .admins()
      .then(setItems)
      .catch((err) => setError(err instanceof ApiError ? err.message : 'не удалось загрузить пользователей'))
      .finally(() => setLoading(false));
  }, []);

  useEffect(load, [load]);

  const run = async (fn: () => Promise<unknown>) => {
    setError('');
    try {
      await fn();
      load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'действие не выполнено');
    }
  };

  const { sorting, sortHeader, sortRows } = useTableSort();
  const columns = useMemo<ColumnDef<AdminUser>[]>(() => [
    { id: 'name', header: () => sortHeader('name', 'Имя'), cell: ({ row }) => row.original.name },
    {
      id: 'role',
      header: () => sortHeader('role', 'Роль'),
      cell: ({ row }) => (
        <Label theme={row.original.role === 'admin' ? 'success' : 'info'} size="s">
          {row.original.role}
        </Label>
      ),
    },
    {
      id: 'state',
      header: () => sortHeader('state', 'Статус'),
      cell: ({ row }) => (
        <Label theme={row.original.disabled ? 'danger' : 'success'} size="s">
          {row.original.disabled ? 'отключён' : 'активен'}
        </Label>
      ),
    },
    {
      id: 'totp',
      header: () => sortHeader('totp', '2FA'),
      cell: ({ row }) =>
        row.original.totpEnabled ? (
          <Label theme="success" size="s">включена</Label>
        ) : (
          <Label theme="unknown" size="s">выключена</Label>
        ),
    },
    { id: 'sessions', header: () => sortHeader('sessions', 'Сессии'), cell: ({ row }) => row.original.sessions },
    { id: 'created', header: () => sortHeader('created', 'Создан'), cell: ({ row }) => formatTime(row.original.createdAt) },
    {
      id: 'actions',
      header: '',
      cell: ({ row }) => (
        <div className="row">
          <div className="icon-actions">
            <Button
              view="flat"
              aria-label="Сменить пароль"
              title="Сменить пароль"
              onClick={() => setResetFor(row.original.name)}
            >
              <Key width={18} height={18} />
            </Button>
            <Button
              view="flat"
              aria-label={row.original.disabled ? 'Включить' : 'Отключить'}
              title={row.original.disabled ? 'Включить' : 'Отключить'}
              onClick={() =>
                void run(() =>
                  api.patchAdmin(row.original.name, { disabled: !row.original.disabled }),
                )
              }
              disabled={row.original.name === me.user.name}
            >
              {row.original.disabled ? <Play width={18} height={18} /> : <Ban width={18} height={18} />}
            </Button>
            <Button
              view="flat"
              aria-label="Сбросить 2FA"
              title="Сбросить 2FA"
              disabled={row.original.name === me.user.name || !row.original.totpEnabled}
              onClick={() => {
                if (window.confirm(`Сбросить 2FA для ${row.original.name}?`)) {
                  void run(() => api.resetAdminTOTP(row.original.name));
                }
              }}
            >
              <Lock width={18} height={18} />
            </Button>
            <Button
              view="flat"
              aria-label="Завершить сессии"
              title="Завершить сессии"
              onClick={() => void run(() => api.revokeAdminSessions(row.original.name))}
              disabled={row.original.name === me.user.name || row.original.sessions === 0}
            >
              <ArrowRightFromSquare width={18} height={18} />
            </Button>
            <Button
              view="flat-danger"
              aria-label="Удалить"
              title="Удалить"
              onClick={() => void run(() => api.deleteAdmin(row.original.name))}
              disabled={row.original.name === me.user.name}
            >
              <TrashBin width={18} height={18} />
            </Button>
          </div>
        </div>
      ),
    },
    settingsColumn<AdminUser>(COLUMN_LABELS),
  ], [sorting]);
  const sorted = useMemo(() => sortRows(items, compareAdmins), [items, sorting]);
  const [columnVisibility, setColumnVisibility] = usePersistentColumnVisibility(
    me.user.name,
    'admins',
  );
  const table = useTable({
    columns,
    data: sorted,
    enableHiding: true,
    state: { columnVisibility },
    onColumnVisibilityChange: setColumnVisibility,
  });

  if (me.user.role !== 'admin') {
    return <Alert theme="warning" title="Нет прав" message="Раздел доступен только администраторам" />;
  }

  return (
    <div className="stack">
      <div className="row">
        <Text variant="header-2">Пользователи админки</Text>
        <div style={{ flex: 1 }} />
        <Button view="action" onClick={() => setCreateOpen(true)}>
          Создать
        </Button>
      </div>
      {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
      {generated && (
        <Alert
          theme="success"
          title="Пароль создан"
          message={`Сохраните и передайте пользователю: ${generated}`}
          onClose={() => setGenerated('')}
        />
      )}
      <Card view="filled" style={{ padding: 8 }}>
        {loading ? (
          <Loader size="m" />
        ) : isMobile ? (
          <div className="user-cards" style={{ padding: 8 }}>
            {sorted.map((u) => (
              <div className="user-card" key={u.name}>
                <div className="row">
                  <Text>{u.name}</Text>
                  <Label theme={u.role === 'admin' ? 'success' : 'info'} size="s">
                    {u.role}
                  </Label>
                  <Label theme={u.disabled ? 'danger' : 'success'} size="s">
                    {u.disabled ? 'отключён' : 'активен'}
                  </Label>
                </div>
                <div className="muted">
                  сессий: {u.sessions} · создан {formatTime(u.createdAt)} · 2FA:{' '}
                  {u.totpEnabled ? 'включена' : 'выключена'}
                </div>
                <div className="icon-actions">
                  <Button view="flat" aria-label="Сменить пароль" title="Сменить пароль" onClick={() => setResetFor(u.name)}>
                    <Key width={18} height={18} />
                  </Button>
                  <Button
                    view="flat"
                    aria-label={u.disabled ? 'Включить' : 'Отключить'}
                    title={u.disabled ? 'Включить' : 'Отключить'}
                    disabled={u.name === me.user.name}
                    onClick={() => void run(() => api.patchAdmin(u.name, { disabled: !u.disabled }))}
                  >
                    {u.disabled ? <Play width={18} height={18} /> : <Ban width={18} height={18} />}
                  </Button>
                  <Button
                    view="flat"
                    aria-label="Сбросить 2FA"
                    title="Сбросить 2FA"
                    disabled={u.name === me.user.name || !u.totpEnabled}
                    onClick={() => {
                      if (window.confirm(`Сбросить 2FA для ${u.name}?`)) {
                        void run(() => api.resetAdminTOTP(u.name));
                      }
                    }}
                  >
                    <Lock width={18} height={18} />
                  </Button>
                  <Button
                    view="flat"
                    aria-label="Завершить сессии"
                    title="Завершить сессии"
                    disabled={u.name === me.user.name || u.sessions === 0}
                    onClick={() => void run(() => api.revokeAdminSessions(u.name))}
                  >
                    <ArrowRightFromSquare width={18} height={18} />
                  </Button>
                  <Button
                    view="flat-danger"
                    aria-label="Удалить"
                    title="Удалить"
                    disabled={u.name === me.user.name}
                    onClick={() => void run(() => api.deleteAdmin(u.name))}
                  >
                    <TrashBin width={18} height={18} />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <Table table={table} emptyContent="Пользователей нет" />
        )}
      </Card>
      <CreateAdminDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={(password) => {
          setCreateOpen(false);
          setGenerated(password);
          load();
        }}
      />
      <ResetPasswordDialog
        name={resetFor}
        onClose={() => setResetFor(null)}
        onDone={(password) => {
          setResetFor(null);
          setGenerated(password);
        }}
      />
    </div>
  );
}

function compareAdmins(a: AdminUser, b: AdminUser, id: string): number {
  switch (id) {
    case 'role':
      return a.role.localeCompare(b.role);
    case 'state':
      return Number(a.disabled) - Number(b.disabled);
    case 'totp':
      return Number(a.totpEnabled) - Number(b.totpEnabled);
    case 'sessions':
      return a.sessions - b.sessions;
    case 'created':
      return a.createdAt.localeCompare(b.createdAt);
    default:
      return a.name.localeCompare(b.name);
  }
}

function CreateAdminDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (password: string) => void;
}) {
  const [name, setName] = useState('');
  const [role, setRole] = useState<'admin' | 'viewer'>('viewer');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()} maxWidth="s">
      <Dialog.Header caption="Новый пользователь админки" />
      <Dialog.Body>
        <div className="stack">
          {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
          <TextInput placeholder="Имя" value={name} onUpdate={setName} />
          <label className="field">
            <span>Роль</span>
            <Select
              aria-label="Роль"
              value={[role]}
              options={[
                { value: 'viewer', content: 'viewer' },
                { value: 'admin', content: 'admin' },
              ]}
              onUpdate={([value]) => setRole((value ?? 'viewer') as 'admin' | 'viewer')}
              width="max"
            />
          </label>
          <TextInput
            placeholder="Пароль (пусто — сгенерировать)"
            type="password"
            value={password}
            onUpdate={setPassword}
          />
        </div>
      </Dialog.Body>
      <Dialog.Footer
        onClickButtonCancel={onClose}
        onClickButtonApply={() => {
          setBusy(true);
          setError('');
          api
            .createAdmin({ name: name.trim(), role, password: password || undefined })
            .then((res) => onCreated(res.password ?? 'задан вручную'))
            .catch((err) => setError(err instanceof ApiError ? err.message : 'не удалось создать'))
            .finally(() => setBusy(false));
        }}
        textButtonApply="Создать"
        textButtonCancel="Отмена"
        propsButtonApply={{ disabled: !name.trim(), loading: busy }}
      />
    </Dialog>
  );
}

function ResetPasswordDialog({
  name,
  onClose,
  onDone,
}: {
  name: string | null;
  onClose: () => void;
  onDone: (password: string) => void;
}) {
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  return (
    <Dialog open={name !== null} onOpenChange={(next) => !next && onClose()} maxWidth="s">
      <Dialog.Header caption={name ? `Новый пароль для ${name}` : ''} />
      <Dialog.Body>
        <div className="stack">
          {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
          <TextInput
            placeholder="Пусто — сгенерировать"
            type="password"
            value={password}
            onUpdate={setPassword}
          />
        </div>
      </Dialog.Body>
      <Dialog.Footer
        onClickButtonCancel={onClose}
        onClickButtonApply={() => {
          if (!name) return;
          api
            .resetAdminPassword(name, password || undefined)
            .then((res) => onDone(res.password ?? 'задан вручную'))
            .catch((err) => setError(err instanceof ApiError ? err.message : 'не удалось сменить пароль'));
        }}
        textButtonApply="Сменить"
        textButtonCancel="Отмена"
      />
    </Dialog>
  );
}
