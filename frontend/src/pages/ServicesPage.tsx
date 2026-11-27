import { useCallback, useEffect, useMemo, useState } from 'react';
import { Alert, Button, Card, Checkbox, Dialog, Label, Loader, Select, Text, TextInput } from '@gravity-ui/uikit';
import { Table, useTable } from '@gravity-ui/table';
import type { ColumnDef } from '@gravity-ui/table/tanstack';
import { Ban, Key, Pencil, Play, TrashBin } from '@gravity-ui/icons';
import { api, ApiError, type Me, type ServiceAccount, type ServiceScope } from '../api';
import { formatTime } from '../format';
import ServiceTokensDialog from './ServiceTokensDialog';

const scopeTitle: Record<ServiceScope, string> = {
  read: 'Чтение',
  write: 'Клиенты',
  admin: 'Администратор',
};

const scopeTheme: Record<ServiceScope, 'info' | 'warning' | 'danger'> = {
  read: 'info',
  write: 'warning',
  admin: 'danger',
};

export default function ServicesPage({ me }: { me: Me }) {
  const [items, setItems] = useState<ServiceAccount[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [editFor, setEditFor] = useState<ServiceAccount | null>(null);
  const [deleteFor, setDeleteFor] = useState<ServiceAccount | null>(null);
  const [tokensFor, setTokensFor] = useState<ServiceAccount | null>(null);

  const load = useCallback(() => {
    setLoading(true);
    api
      .services()
      .then(setItems)
      .catch((err) =>
        setError(err instanceof ApiError ? err.message : 'не удалось загрузить сервисы'),
      )
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

  const columns = useMemo<ColumnDef<ServiceAccount>[]>(
    () => [
      {
        id: 'name',
        header: 'Имя',
        cell: ({ row }) => <Text variant="subheader-1">{row.original.name}</Text>,
      },
      {
        id: 'description',
        header: 'Описание',
        cell: ({ row }) => row.original.description || '—',
      },
      {
        id: 'scope',
        header: 'Права',
        cell: ({ row }) => (
          <Label theme={scopeTheme[row.original.scope]} size="s">
            {scopeTitle[row.original.scope]}
          </Label>
        ),
      },
      { id: 'tokens', header: 'Токенов', cell: ({ row }) => row.original.tokens },
      {
        id: 'status',
        header: 'Статус',
        cell: ({ row }) => (
          <Label theme={row.original.disabled ? 'danger' : 'success'} size="s">
            {row.original.disabled ? 'отключён' : 'активен'}
          </Label>
        ),
      },
      {
        id: 'created',
        header: 'Создан',
        cell: ({ row }) => formatTime(row.original.createdAt),
      },
      {
        id: 'actions',
        header: '',
        cell: ({ row }) => (
          <div className="icon-actions">
            <Button
              view="flat"
              aria-label={`Токены ${row.original.name}`}
              title="Токены"
              onClick={() => setTokensFor(row.original)}
            >
              <Key width={18} height={18} />
            </Button>
            <Button
              view="flat"
              aria-label={`Изменить ${row.original.name}`}
              title="Изменить"
              onClick={() => setEditFor(row.original)}
            >
              <Pencil width={18} height={18} />
            </Button>
            <Button
              view="flat"
              aria-label={
                row.original.disabled
                  ? `Включить ${row.original.name}`
                  : `Отключить ${row.original.name}`
              }
              title={row.original.disabled ? 'Включить' : 'Отключить'}
              onClick={() =>
                void run(() =>
                  api.updateService(row.original.name, { disabled: !row.original.disabled }),
                )
              }
            >
              {row.original.disabled ? <Play width={18} height={18} /> : <Ban width={18} height={18} />}
            </Button>
            <Button
              view="flat"
              aria-label={`Удалить ${row.original.name}`}
              title="Удалить"
              onClick={() => setDeleteFor(row.original)}
            >
              <TrashBin width={18} height={18} />
            </Button>
          </div>
        ),
      },
    ],
    [],
  );

  const table = useTable({ columns, data: items });

  if (me.user.role !== 'admin') {
    return <Alert theme="danger" message="Раздел доступен только администратору" />;
  }

  return (
    <div className="stack">
      <div className="row">
        <Text variant="header-2">Сервисные аккаунты</Text>
        <div style={{ flex: 1 }} />
        <Button view="action" onClick={() => setCreateOpen(true)}>
          Создать сервис
        </Button>
      </div>
      <Text className="muted">
        Сервисные аккаунты нужны для скриптов и интеграций. Токен передаётся в
        заголовке Authorization: Bearer и действует только в разрешённых сетях.
      </Text>
      {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
      <Card view="filled" style={{ padding: 8 }}>
        {loading ? (
          <Loader size="m" />
        ) : items.length === 0 ? (
          <Text className="muted">Сервисных аккаунтов пока нет.</Text>
        ) : (
          <div className="table-scroll">
            <Table table={table} />
          </div>
        )}
      </Card>

      <ServiceDialog
        open={createOpen || editFor !== null}
        service={editFor}
        onClose={() => {
          setCreateOpen(false);
          setEditFor(null);
        }}
        onSaved={() => {
          setCreateOpen(false);
          setEditFor(null);
          load();
        }}
      />
      <ServiceTokensDialog
        service={tokensFor}
        open={tokensFor !== null}
        onClose={() => setTokensFor(null)}
        onChanged={load}
      />
      <DeleteServiceDialog
        service={deleteFor}
        onClose={() => setDeleteFor(null)}
        onDeleted={() => {
          setDeleteFor(null);
          load();
        }}
      />
    </div>
  );
}

function ServiceDialog({
  open,
  service,
  onClose,
  onSaved,
}: {
  open: boolean;
  service: ServiceAccount | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [scope, setScope] = useState<ServiceScope>('read');
  const [disabled, setDisabled] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!open) return;
    setName(service?.name ?? '');
    setDescription(service?.description ?? '');
    setScope(service?.scope ?? 'read');
    setDisabled(service?.disabled ?? false);
    setError('');
  }, [open, service]);

  const submit = async () => {
    setBusy(true);
    setError('');
    try {
      if (service) {
        await api.updateService(service.name, {
          description,
          scope,
          disabled,
        });
      } else {
        await api.createService({ name, description, scope });
      }
      onSaved();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось сохранить сервис');
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()} maxWidth="s">
      <Dialog.Header caption={service ? 'Изменить сервис' : 'Новый сервис'} />
      <Dialog.Body>
        <div className="stack">
          {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
          <TextInput
            placeholder="Имя"
            value={name}
            onUpdate={setName}
            disabled={Boolean(service)}
          />
          <TextInput
            placeholder="Описание"
            value={description}
            onUpdate={setDescription}
          />
          <label className="field">
            <span>Права</span>
            <Select
              aria-label="Права"
              value={[scope]}
              options={[
                { value: 'read', content: 'Чтение — только просмотр' },
                { value: 'write', content: 'Клиенты — управление клиентами' },
                { value: 'admin', content: 'Администратор — полный доступ' },
              ]}
              onUpdate={([value]) => setScope((value ?? 'read') as ServiceScope)}
              width="max"
            />
          </label>
          <Text className="muted">
            Смена прав действует на все токены сервиса сразу.
          </Text>
          {service && (
            <Checkbox
              controlProps={{ 'aria-label': 'Отключён' }}
              checked={disabled}
              onUpdate={setDisabled}
              content="Отключён"
            />
          )}
        </div>
      </Dialog.Body>
      <Dialog.Footer
        onClickButtonCancel={onClose}
        onClickButtonApply={() => void submit()}
        textButtonApply={service ? 'Сохранить' : 'Создать'}
        textButtonCancel="Отмена"
        propsButtonApply={{ loading: busy, disabled: !name.trim() }}
      />
    </Dialog>
  );
}

function DeleteServiceDialog({
  service,
  onClose,
  onDeleted,
}: {
  service: ServiceAccount | null;
  onClose: () => void;
  onDeleted: () => void;
}) {
  const [confirm, setConfirm] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (service) {
      setConfirm('');
      setError('');
    }
  }, [service]);

  const remove = async () => {
    if (!service) return;
    setBusy(true);
    setError('');
    try {
      await api.deleteService(service.name);
      onDeleted();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось удалить сервис');
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={service !== null} onOpenChange={(next) => !next && onClose()} maxWidth="s">
      <Dialog.Header caption="Удалить сервис" />
      <Dialog.Body>
        <div className="stack">
          {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
          <Text>
            Все токены сервиса «{service?.name}» перестанут работать. Введите имя для
            подтверждения.
          </Text>
          <TextInput
            placeholder="Имя сервиса"
            value={confirm}
            onUpdate={setConfirm}
          />
        </div>
      </Dialog.Body>
      <Dialog.Footer
        onClickButtonCancel={onClose}
        onClickButtonApply={() => void remove()}
        textButtonApply="Удалить"
        textButtonCancel="Отмена"
        propsButtonApply={{
          loading: busy,
          view: 'outlined-danger',
          disabled: confirm !== service?.name,
        }}
      />
    </Dialog>
  );
}
