import { useCallback, useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import {
  Alert,
  Button,
  Card,
  Dialog,
  Label,
  Loader,
  Text,
  TextInput,
} from '@gravity-ui/uikit';
import { Table, useTable } from '@gravity-ui/table';
import type { ColumnDef } from '@gravity-ui/table/tanstack';
import { api, ApiError, type IPHistoryItem, type Me, type ProxyUser } from '../api';
import { formatBytes } from '../format';

export default function UserPage({ me }: { me: Me }) {
  const { name = '' } = useParams();
  const navigate = useNavigate();
  const [user, setUser] = useState<ProxyUser | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [rotateOpen, setRotateOpen] = useState(false);
  const [renameOpen, setRenameOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [maxIps, setMaxIps] = useState('');
  const [maxConns, setMaxConns] = useState('');
  const [clearIps, setClearIps] = useState(false);
  const [clearConns, setClearConns] = useState(false);
  const [quotaGB, setQuotaGB] = useState('');
  const [period, setPeriod] = useState('');
  const [rateUp, setRateUp] = useState('');
  const [rateDown, setRateDown] = useState('');
  const [expiration, setExpiration] = useState('');
  const [sourceDeny, setSourceDeny] = useState('');
  const [clearQuota, setClearQuota] = useState(false);
  const [clearRates, setClearRates] = useState(false);
  const [clearExpiration, setClearExpiration] = useState(false);
  const [clearSource, setClearSource] = useState(false);
  const [note, setNote] = useState('');
  const [noteSaved, setNoteSaved] = useState(false);
  const [copied, setCopied] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const value = await api.user(name);
      setUser(value);
      setMaxIps(value.maxUniqueIps?.toString() ?? '');
      setMaxConns(value.maxTcpConns?.toString() ?? '');
      setQuotaGB(value.dataQuotaBytes ? (value.dataQuotaBytes / 1024 ** 3).toFixed(2) : '');
      setPeriod(value.quotaPeriod ?? '');
      setRateUp(value.rateLimitUpBps ? (value.rateLimitUpBps / 1000).toString() : '');
      setRateDown(value.rateLimitDownBps ? (value.rateLimitDownBps / 1000).toString() : '');
      setExpiration(value.expiration ? value.expiration.slice(0, 16) : '');
      setSourceDeny((value.sourceDeny ?? []).join('\n'));
      setNote(value.note ?? '');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось загрузить пользователя');
    } finally {
      setLoading(false);
    }
  }, [name]);

  useEffect(() => {
    void load();
  }, [load]);

  const run = async (action: () => Promise<unknown>) => {
    setBusy(true);
    setError('');
    try {
      await action();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'действие не выполнено');
    } finally {
      setBusy(false);
      await load();
    }
  };

  if (loading) return <Loader size="l" />;
  if (!user) {
    return (
      <div className="stack">
        {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
        <Link to="/">← к списку</Link>
      </div>
    );
  }
  const isAdmin = me.user.role === 'admin';

  return (
    <div className="stack">
      <div className="row">
        <Text variant="header-2">{user.username}</Text>
        <Label theme={user.enabled ? 'success' : 'danger'} size="s">
          {user.enabled ? 'включён' : 'заблокирован'}
        </Label>
        <div style={{ flex: 1 }} />
        <Link to="/">← к списку</Link>
      </div>
      {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}

      <Card view="filled" style={{ padding: 16 }}>
        <div className="stack">
          <Text>Соединения: {user.currentConnections}</Text>
          <Text>Активные IP: {user.activeIps.join(', ') || '—'}</Text>
          <Text>Трафик: {formatBytes(user.totalOctets)}</Text>
          <Text>Лимит IP: {user.maxUniqueIps ?? 'нет'}</Text>
          <Text>Лимит соединений: {user.maxTcpConns ?? 'нет'}</Text>
        </div>
      </Card>

      <Card view="filled" style={{ padding: 16 }}>
        <div className="stack">
          <div className="row">
            <Text variant="header-1">Устройства</Text>
            {user.risk > 0 && (
              <Label theme={user.risk === 2 ? 'danger' : 'warning'} size="s">
                {user.risk === 2 ? 'высокий риск' : 'внимание'}
              </Label>
            )}
          </div>
          {user.risk > 0 && (
            <Alert
              theme={user.risk === 2 ? 'danger' : 'warning'}
              message="Столько одновременных адресов не похоже на обычное число устройств. Проверьте, не передан ли ключ."
            />
          )}
          <Text>Сейчас активно: {user.activeUniqueIps}</Text>
          <Text>Уникальных за 24 часа: {user.ips24h}</Text>
          <Text>Уникальных за 7 дней: {user.ips7d}</Text>
          <Text>Новых за 24 часа: {user.new24h}</Text>
          <Text>Пик за сутки: {user.maxToday}</Text>
          <Text>Трафик за сутки: {formatBytes(user.trafficToday)}</Text>
          {isAdmin && (
            <div className="row">
              <a href={api.historyCSVUrl(user.username)} download={`ips-${user.username}.csv`}>
                Экспорт истории CSV
              </a>
              <Button view="outlined-danger" onClick={() => setHistoryOpen(true)}>
                Удалить историю
              </Button>
            </div>
          )}
          {isAdmin && user.ipHistory && user.ipHistory.length > 0 && <IPHistoryTable items={user.ipHistory} />}
        </div>
      </Card>

      {isAdmin && (
        <Card view="filled" style={{ padding: 16 }}>
          <div className="stack">
            <Text variant="header-1">Заметка</Text>
            <TextInput
              placeholder="например: телефон Юли"
              value={note}
              onUpdate={(value) => {
                setNote(value);
                setNoteSaved(false);
              }}
            />
            <div className="row">
              <Button
                view="outlined"
                onClick={() =>
                  void api.setNote(user.username, note).then(() => setNoteSaved(true))
                }
              >
                Сохранить заметку
              </Button>
              {noteSaved && <Text color="positive">сохранено</Text>}
            </div>
          </div>
        </Card>
      )}

      {isAdmin && user.link && (
        <Card view="filled" style={{ padding: 16 }}>
          <div className="stack">
            <Text variant="header-1">Ссылка</Text>
            <div className="link">{user.link}</div>
            <Alert theme="warning" message="Ссылка равна паролю: не публикуйте её." />
            <div className="row">
              <Button
                view="outlined"
                onClick={() =>
                  void navigator.clipboard.writeText(user.link!).then(() => setCopied(true))
                }
              >
                Скопировать
              </Button>
              {copied && <Text color="positive">скопировано</Text>}
            </div>
            <a href={api.qrUrl(user.username)} download={`qr-${user.username}.png`}>
              <img className="qr" src={api.qrUrl(user.username)} alt="QR" width={256} height={256} />
            </a>
          </div>
        </Card>
      )}

      {isAdmin && (
        <>
          <Card view="filled" style={{ padding: 16 }}>
            <div className="stack">
              <Text variant="header-1">Действия</Text>
              <div className="row">
                <Button view="outlined" onClick={() => setRotateOpen(true)}>
                  Перевыпустить секрет
                </Button>
                <Button
                  view={user.enabled ? 'outlined-danger' : 'outlined-success'}
                  onClick={() =>
                    void run(() => (user.enabled ? api.disable(user.username) : api.enable(user.username)))
                  }
                  loading={busy}
                >
                  {user.enabled ? 'Заблокировать' : 'Разблокировать'}
                </Button>
                <Button view="outlined" onClick={() => setRenameOpen(true)}>
                  Переименовать
                </Button>
                <Button view="outlined-danger" onClick={() => setDeleteOpen(true)}>
                  Удалить
                </Button>
              </div>
            </div>
          </Card>

          <Card view="filled" style={{ padding: 16 }}>
            <form
              className="stack"
              onSubmit={(event) => {
                event.preventDefault();
                void run(() =>
                  api.updateLimits(user.username, {
                    maxUniqueIps: maxIps ? Number(maxIps) : undefined,
                    maxTcpConns: maxConns ? Number(maxConns) : undefined,
                    clearMaxUniqueIps: clearIps,
                    clearMaxTcpConns: clearConns,
                    dataQuotaBytes: quotaGB ? Math.round(Number(quotaGB) * 1024 ** 3) : undefined,
                    quotaPeriod: period || undefined,
                    clearQuota,
                    rateLimitUpBps: rateUp ? Math.round(Number(rateUp) * 1000) : undefined,
                    rateLimitDownBps: rateDown ? Math.round(Number(rateDown) * 1000) : undefined,
                    clearRates,
                    expiration: expiration ? new Date(expiration).toISOString() : undefined,
                    clearExpiration,
                    sourceDeny: sourceDeny
                      .split(/[\n,]+/)
                      .map((v) => v.trim())
                      .filter(Boolean),
                    clearSourceDeny: clearSource,
                  }),
                );
              }}
            >
              <Text variant="header-1">Лимиты</Text>
              <TextInput
                placeholder="Максимум уникальных IP"
                value={maxIps}
                onUpdate={setMaxIps}
                disabled={clearIps}
              />
              <label>
                <input type="checkbox" checked={clearIps} onChange={(e) => setClearIps(e.target.checked)} /> снять
                лимит IP
              </label>
              <TextInput
                placeholder="Максимум соединений"
                value={maxConns}
                onUpdate={setMaxConns}
                disabled={clearConns}
              />
              <label>
                <input type="checkbox" checked={clearConns} onChange={(e) => setClearConns(e.target.checked)} />{' '}
                снять лимит соединений
              </label>
              <Text variant="subheader-1">Квота трафика</Text>
              <TextInput
                placeholder="Квота, ГиБ (пусто — без квоты)"
                value={quotaGB}
                onUpdate={setQuotaGB}
                disabled={clearQuota}
              />
              <label>
                Период:{' '}
                <select value={period} onChange={(e) => setPeriod(e.target.value)} disabled={clearQuota}>
                  <option value="">без сброса</option>
                  <option value="daily">сутки</option>
                  <option value="weekly">неделя</option>
                  <option value="monthly">месяц</option>
                </select>
              </label>
              <label>
                <input type="checkbox" checked={clearQuota} onChange={(e) => setClearQuota(e.target.checked)} /> снять
                квоту
              </label>

              <Text variant="subheader-1">Скорость</Text>
              <TextInput
                placeholder="Вверх, Кбит/с"
                value={rateUp}
                onUpdate={setRateUp}
                disabled={clearRates}
              />
              <TextInput
                placeholder="Вниз, Кбит/с"
                value={rateDown}
                onUpdate={setRateDown}
                disabled={clearRates}
              />
              <label>
                <input type="checkbox" checked={clearRates} onChange={(e) => setClearRates(e.target.checked)} /> снять
                лимиты скорости
              </label>

              <Text variant="subheader-1">Срок действия</Text>
              <input
                type="datetime-local"
                value={expiration}
                disabled={clearExpiration}
                onChange={(e) => setExpiration(e.target.value)}
              />
              <label>
                <input
                  type="checkbox"
                  checked={clearExpiration}
                  onChange={(e) => setClearExpiration(e.target.checked)}
                />{' '}
                без срока
              </label>

              <Text variant="subheader-1">Запрещённые сети (source_deny)</Text>
              <textarea
                rows={3}
                value={sourceDeny}
                disabled={clearSource}
                onChange={(e) => setSourceDeny(e.target.value)}
                placeholder="по одному CIDR на строку"
              />
              <label>
                <input type="checkbox" checked={clearSource} onChange={(e) => setClearSource(e.target.checked)} />{' '}
                очистить список
              </label>

              <div className="row">
                <Button view="action" type="submit" loading={busy}>
                  Сохранить
                </Button>
                <Button view="outlined" onClick={() => void run(() => api.resetQuota(user.username))}>
                  Сбросить квоту
                </Button>
              </div>
            </form>
          </Card>
        </>
      )}

      <ConfirmDialog
        open={rotateOpen}
        title="Перевыпустить секрет"
        description={`Введите имя ${user.username}. Старая ссылка перестанет работать.`}
        confirmWord={user.username}
        applyText="Перевыпустить"
        onClose={() => setRotateOpen(false)}
        onApply={() => {
          setRotateOpen(false);
          void run(() => api.rotate(user.username));
        }}
      />
      <ConfirmDialog
        open={deleteOpen}
        title="Удалить пользователя"
        description={`Введите имя ${user.username}. Действие необратимо.`}
        confirmWord={user.username}
        applyText="Удалить"
        onClose={() => setDeleteOpen(false)}
        onApply={() => {
          setDeleteOpen(false);
          void run(async () => {
            await api.remove(user.username);
            navigate('/');
          });
        }}
      />
      <ConfirmDialog
        open={historyOpen}
        title="Удалить историю IP"
        description={`Введите имя ${user.username}. История адресов и трафика будет удалена, настройки пользователя останутся.`}
        confirmWord={user.username}
        applyText="Удалить историю"
        onClose={() => setHistoryOpen(false)}
        onApply={() => {
          setHistoryOpen(false);
          void run(() => api.deleteHistory(user.username));
        }}
      />
      <RenameDialog
        open={renameOpen}
        current={user.username}
        onClose={() => setRenameOpen(false)}
        onApply={async (next) => {
          await api.rename(user.username, next);
          setRenameOpen(false);
          navigate(`/users/${encodeURIComponent(next)}`);
        }}
      />
    </div>
  );
}

function IPHistoryTable({ items }: { items: IPHistoryItem[] }) {
  const columns: ColumnDef<IPHistoryItem>[] = [
    { id: 'ip', header: 'IP', cell: ({ row }) => row.original.ip },
    { id: 'first', header: 'Впервые', cell: ({ row }) => new Date(row.original.firstSeen).toLocaleString('ru-RU') },
    { id: 'last', header: 'Последний раз', cell: ({ row }) => new Date(row.original.lastSeen).toLocaleString('ru-RU') },
    { id: 'count', header: 'Наблюдений', cell: ({ row }) => row.original.count },
  ];
  const table = useTable({ columns, data: items });
  return <Table table={table} />;
}

function ConfirmDialog({
  open,
  title,
  description,
  confirmWord,
  applyText,
  onClose,
  onApply,
}: {
  open: boolean;
  title: string;
  description: string;
  confirmWord: string;
  applyText: string;
  onClose: () => void;
  onApply: () => void;
}) {
  const [value, setValue] = useState('');
  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()} maxWidth="s">
      <Dialog.Header caption={title} />
      <Dialog.Body>
        <div className="stack">
          <Text>{description}</Text>
          <TextInput placeholder="Имя" value={value} onUpdate={setValue} />
        </div>
      </Dialog.Body>
      <Dialog.Footer
        onClickButtonCancel={onClose}
        onClickButtonApply={() => {
          if (value === confirmWord) onApply();
        }}
        textButtonApply={applyText}
        textButtonCancel="Отмена"
        propsButtonApply={{ disabled: value !== confirmWord, view: 'outlined-danger' }}
      />
    </Dialog>
  );
}

function RenameDialog({
  open,
  current,
  onClose,
  onApply,
}: {
  open: boolean;
  current: string;
  onClose: () => void;
  onApply: (next: string) => Promise<void>;
}) {
  const [value, setValue] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()} maxWidth="s">
      <Dialog.Header caption="Переименовать" />
      <Dialog.Body>
        <div className="stack">
          {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
          <Text>Текущее имя: {current}. Ссылка не изменится.</Text>
          <TextInput placeholder="Новое имя" value={value} onUpdate={setValue} />
        </div>
      </Dialog.Body>
      <Dialog.Footer
        onClickButtonCancel={onClose}
        onClickButtonApply={() => {
          setBusy(true);
          setError('');
          onApply(value.trim())
            .catch((err) => setError(err instanceof ApiError ? err.message : 'не удалось переименовать'))
            .finally(() => setBusy(false));
        }}
        textButtonApply="Переименовать"
        textButtonCancel="Отмена"
        propsButtonApply={{ disabled: !value.trim(), loading: busy }}
      />
    </Dialog>
  );
}
