import { useCallback, useEffect, useMemo, useState } from 'react';
import { Alert, Button, Dialog, Loader, Text, TextInput } from '@gravity-ui/uikit';
import { Table, useTable } from '@gravity-ui/table';
import type { ColumnDef } from '@gravity-ui/table/tanstack';
import { api, ApiError, type IssuedServiceToken, type ServiceAccount, type ServiceToken } from '../api';
import { formatTime } from '../format';

export default function ServiceTokensDialog({
  service,
  open,
  onClose,
  onChanged,
}: {
  service: ServiceAccount | null;
  open: boolean;
  onClose: () => void;
  onChanged: () => void;
}) {
  const [tokens, setTokens] = useState<ServiceToken[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [formOpen, setFormOpen] = useState(false);
  const [label, setLabel] = useState('');
  const [days, setDays] = useState('90');
  const [busy, setBusy] = useState(false);
  const [issued, setIssued] = useState<IssuedServiceToken | null>(null);
  const [copied, setCopied] = useState(false);

  const load = useCallback(() => {
    if (!service) return;
    setLoading(true);
    api
      .serviceTokens(service.name)
      .then(setTokens)
      .catch((err) => setError(err instanceof ApiError ? err.message : 'не удалось загрузить токены'))
      .finally(() => setLoading(false));
  }, [service]);

  useEffect(() => {
    if (!open) return;
    setError('');
    setFormOpen(false);
    setIssued(null);
    setLabel('');
    setDays('90');
    setCopied(false);
    load();
  }, [open, load]);

  const issue = async () => {
    if (!service) return;
    const value = Number(days);
    if (!Number.isInteger(value) || value < 0 || value > 3650) {
      setError('срок: 0 или от 1 до 3650 дней');
      return;
    }
    setBusy(true);
    setError('');
    try {
      const result = await api.issueServiceToken(service.name, {
        label: label.trim() || undefined,
        expiresInDays: value,
      });
      setIssued(result);
      setFormOpen(false);
      setLabel('');
      setCopied(false);
      onChanged();
      load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось выпустить токен');
    } finally {
      setBusy(false);
    }
  };

  const revoke = async (token: ServiceToken) => {
    if (!service) return;
    if (!window.confirm(`Отозвать токен «${token.label || token.id}»?`)) return;
    setError('');
    try {
      await api.revokeServiceToken(service.name, token.id);
      onChanged();
      load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось отозвать токен');
    }
  };

  const copy = async () => {
    if (!issued) return;
    await navigator.clipboard.writeText(issued.token);
    setCopied(true);
  };

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()} maxWidth="l">
      <Dialog.Header caption={`Токены: ${service?.name ?? ''}`} />
      <Dialog.Body>
        <div className="stack">
          {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
          {service?.disabled && (
            <Alert theme="warning" message="Сервис отключён: его токены сейчас не работают." />
          )}
          {issued ? (
            <div className="stack">
              <Alert
                theme="success"
                message="Токен выпущен. Скопируйте его сейчас: второй раз он показывается один раз."
              />
              <Text className="muted">Токен:</Text>
              <code style={{ wordBreak: 'break-all' }}>{issued.token}</code>
              <Text className="muted">Заголовок для API:</Text>
              <code style={{ wordBreak: 'break-all' }}>{`Authorization: Bearer ${issued.token}`}</code>
              <div className="row">
                <Button view="action" onClick={() => void copy()}>
                  Скопировать
                </Button>
                <Button view="outlined" onClick={() => setIssued(null)}>
                  Готово
                </Button>
                {copied && <Text color="positive">скопировано</Text>}
              </div>
            </div>
          ) : (
            <>
              <div className="row">
                <Button view="action" onClick={() => setFormOpen(true)}>
                  Выпустить токен
                </Button>
              </div>
              {formOpen && (
                <div className="stack" style={{ border: '1px solid var(--g-color-line-generic)', padding: 12 }}>
                  <TextInput placeholder="Метка (необязательно)" value={label} onUpdate={setLabel} />
                  <label className="field">
                    <span>Срок, дней (0 — бессрочно)</span>
                    <TextInput placeholder="90" value={days} onUpdate={setDays} />
                  </label>
                  <div className="row">
                    <Button view="action" loading={busy} onClick={() => void issue()}>
                      Выпустить
                    </Button>
                    <Button view="outlined" onClick={() => setFormOpen(false)}>
                      Отмена
                    </Button>
                  </div>
                </div>
              )}
              {loading ? (
                <Loader size="m" />
              ) : tokens.length === 0 ? (
                <Text className="muted">Токенов пока нет.</Text>
              ) : (
                <TokensTable tokens={tokens} onRevoke={(token) => void revoke(token)} />
              )}
            </>
          )}
        </div>
      </Dialog.Body>
      <Dialog.Footer onClickButtonCancel={onClose} textButtonCancel="Закрыть" />
    </Dialog>
  );
}

function TokensTable({
  tokens,
  onRevoke,
}: {
  tokens: ServiceToken[];
  onRevoke: (token: ServiceToken) => void;
}) {
  const columns = useMemo<ColumnDef<ServiceToken>[]>(
    () => [
      { id: 'label', header: 'Метка', cell: ({ row }) => row.original.label || '—' },
      { id: 'id', header: 'ID', cell: ({ row }) => row.original.prefix },
      {
        id: 'created',
        header: 'Создан',
        cell: ({ row }) => formatTime(row.original.createdAt),
      },
      {
        id: 'expires',
        header: 'Срок',
        cell: ({ row }) =>
          row.original.expiresAt ? formatTime(row.original.expiresAt) : 'бессрочно',
      },
      {
        id: 'lastUsed',
        header: 'Последнее использование',
        cell: ({ row }) =>
          row.original.lastUsedAt ? formatTime(row.original.lastUsedAt) : 'не использовался',
      },
      {
        id: 'actions',
        header: '',
        cell: ({ row }) => (
          <Button
            view="flat"
            aria-label={`Отозвать ${row.original.label || row.original.id}`}
            onClick={() => onRevoke(row.original)}
          >
            Отозвать
          </Button>
        ),
      },
    ],
    [onRevoke],
  );
  const table = useTable({ columns, data: tokens });
  return (
    <div className="table-scroll">
      <Table table={table} />
    </div>
  );
}
