import { useCallback, useEffect, useState } from 'react';
import { Alert, Button, Dialog, Loader, Text, TextInput } from '@gravity-ui/uikit';
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
                <div className="table-scroll">
                  <table>
                    <thead>
                      <tr>
                        <th>Метка</th>
                        <th>ID</th>
                        <th>Создан</th>
                        <th>Срок</th>
                        <th>Последнее использование</th>
                        <th></th>
                      </tr>
                    </thead>
                    <tbody>
                      {tokens.map((token) => (
                        <tr key={token.id}>
                          <td>{token.label || '—'}</td>
                          <td>{token.prefix}</td>
                          <td>{formatTime(token.createdAt)}</td>
                          <td>{token.expiresAt ? formatTime(token.expiresAt) : 'бессрочно'}</td>
                          <td>{token.lastUsedAt ? formatTime(token.lastUsedAt) : 'не использовался'}</td>
                          <td>
                            <Button
                              view="flat"
                              aria-label={`Отозвать ${token.label || token.id}`}
                              onClick={() => void revoke(token)}
                            >
                              Отозвать
                            </Button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </>
          )}
        </div>
      </Dialog.Body>
      <Dialog.Footer onClickButtonCancel={onClose} textButtonCancel="Закрыть" />
    </Dialog>
  );
}
