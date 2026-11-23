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
import { api, ApiError, type Me, type ProxyUser } from '../api';
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

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const value = await api.user(name);
      setUser(value);
      setMaxIps(value.maxUniqueIps?.toString() ?? '');
      setMaxConns(value.maxTcpConns?.toString() ?? '');
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

      {isAdmin && user.link && (
        <Card view="filled" style={{ padding: 16 }}>
          <div className="stack">
            <Text variant="header-1">Ссылка</Text>
            <div className="link">{user.link}</div>
            <Alert theme="warning" message="Ссылка равна паролю: не публикуйте её." />
            <div className="row">
              <Button view="outlined" onClick={() => void navigator.clipboard.writeText(user.link!)}>
                Скопировать
              </Button>
            </div>
            <img className="qr" src={api.qrUrl(user.username)} alt="QR" width={256} height={256} />
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
              <div>
                <Button view="action" type="submit" loading={busy}>
                  Сохранить
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
        onApply={() => void run(() => api.rotate(user.username))}
      />
      <ConfirmDialog
        open={deleteOpen}
        title="Удалить пользователя"
        description={`Введите имя ${user.username}. Действие необратимо.`}
        confirmWord={user.username}
        applyText="Удалить"
        onClose={() => setDeleteOpen(false)}
        onApply={() => {
          void run(async () => {
            await api.remove(user.username);
            navigate('/');
          });
        }}
      />
      <RenameDialog
        open={renameOpen}
        current={user.username}
        onClose={() => setRenameOpen(false)}
        onApply={async (next) => {
          await api.rename(user.username, next);
          navigate(`/users/${encodeURIComponent(next)}`);
        }}
      />
    </div>
  );
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
