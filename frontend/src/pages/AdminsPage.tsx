import { useCallback, useEffect, useState } from 'react';
import { Alert, Button, Card, Dialog, Label, Loader, Text, TextInput } from '@gravity-ui/uikit';
import { Table, useTable } from '@gravity-ui/table';
import type { ColumnDef } from '@gravity-ui/table/tanstack';
import { api, ApiError, type AdminUser, type Me } from '../api';
import { formatTime } from '../format';

export default function AdminsPage({ me }: { me: Me }) {
  const [items, setItems] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [generated, setGenerated] = useState('');
  const [resetFor, setResetFor] = useState<string | null>(null);

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

  const columns: ColumnDef<AdminUser>[] = [
    { id: 'name', header: 'Имя', cell: ({ row }) => row.original.name },
    {
      id: 'role',
      header: 'Роль',
      cell: ({ row }) => (
        <Label theme={row.original.role === 'admin' ? 'success' : 'info'} size="s">
          {row.original.role}
        </Label>
      ),
    },
    {
      id: 'state',
      header: 'Статус',
      cell: ({ row }) => (
        <Label theme={row.original.disabled ? 'danger' : 'success'} size="s">
          {row.original.disabled ? 'отключён' : 'активен'}
        </Label>
      ),
    },
    { id: 'sessions', header: 'Сессии', cell: ({ row }) => row.original.sessions },
    { id: 'created', header: 'Создан', cell: ({ row }) => formatTime(row.original.createdAt) },
    {
      id: 'actions',
      header: '',
      cell: ({ row }) => (
        <div className="row">
          <Button view="flat" onClick={() => setResetFor(row.original.name)}>
            Пароль
          </Button>
          <Button
            view="flat"
            onClick={() =>
              void run(() =>
                api.patchAdmin(row.original.name, { disabled: !row.original.disabled }),
              )
            }
            disabled={row.original.name === me.user.name}
          >
            {row.original.disabled ? 'Включить' : 'Отключить'}
          </Button>
          <Button
            view="flat"
            onClick={() => void run(() => api.revokeAdminSessions(row.original.name))}
            disabled={row.original.name === me.user.name || row.original.sessions === 0}
          >
            Сессии
          </Button>
          <Button
            view="flat-danger"
            onClick={() => void run(() => api.deleteAdmin(row.original.name))}
            disabled={row.original.name === me.user.name}
          >
            Удалить
          </Button>
        </div>
      ),
    },
  ];
  const table = useTable({ columns, data: items });

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
        {loading ? <Loader size="m" /> : <Table table={table} emptyContent="Пользователей нет" />}
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
          <label>
            Роль:{' '}
            <select value={role} onChange={(e) => setRole(e.target.value as 'admin' | 'viewer')}>
              <option value="viewer">viewer</option>
              <option value="admin">admin</option>
            </select>
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
