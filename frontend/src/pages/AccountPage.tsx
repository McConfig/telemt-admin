import { useEffect, useMemo, useState } from 'react';
import { Alert, Button, Card, Loader, Text, TextInput } from '@gravity-ui/uikit';
import { Table, useTable } from '@gravity-ui/table';
import type { ColumnDef } from '@gravity-ui/table/tanstack';
import {
  api,
  ApiError,
  type SessionItem,
  type TOTPSetup,
  type TOTPStatus,
} from '../api';
import { formatTime } from '../format';
import { useTableSort } from '../tableSort';
import { useMedia } from '../useMedia';

function compareSessions(a: SessionItem, b: SessionItem, id: string): number {
  switch (id) {
    case 'ip':
      return a.ip.localeCompare(b.ip);
    case 'at':
      return a.createdAt.localeCompare(b.createdAt);
    default:
      return (a.userAgent ?? '').localeCompare(b.userAgent ?? '');
  }
}

export default function AccountPage() {
  const [sessions, setSessions] = useState<SessionItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [totp, setTotp] = useState<TOTPStatus | null>(null);
  const [setup, setSetup] = useState<TOTPSetup | null>(null);
  const [totpCode, setTotpCode] = useState('');
  const [totpPassword, setTotpPassword] = useState('');
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([]);
  const [totpBusy, setTotpBusy] = useState(false);
  const isMobile = useMedia('(max-width: 760px)');

  const load = () => {
    setLoading(true);
    api
      .totpStatus()
      .then(setTotp)
      .catch(() => setTotp(null));
    api
      .sessions()
      .then(setSessions)
      .catch((err) => setError(err instanceof ApiError ? err.message : 'не удалось загрузить сессии'))
      .finally(() => setLoading(false));
  };

  useEffect(load, []);

  const { sorting, sortHeader, sortRows } = useTableSort();
  const columns = useMemo<ColumnDef<SessionItem>[]>(() => [
    { id: 'ua', header: () => sortHeader('ua', 'Устройство'), cell: ({ row }) => row.original.userAgent || '—' },
    { id: 'ip', header: () => sortHeader('ip', 'IP'), cell: ({ row }) => row.original.ip },
    { id: 'at', header: () => sortHeader('at', 'Вход'), cell: ({ row }) => formatTime(row.original.createdAt) },
    {
      id: 'action',
      header: '',
      cell: ({ row }) =>
        row.original.current ? (
          <Text color="secondary">текущая</Text>
        ) : (
          <Button view="flat" onClick={() => void api.revoke(row.original.id).then(load)}>
            Завершить
          </Button>
        ),
    },
  ], [sorting]);

  const changePassword = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError('');
    setMessage('');
    try {
      await api.changePassword(current, next);
      setCurrent('');
      setNext('');
      setMessage('Пароль изменён, другие сессии завершены');
      load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось сменить пароль');
    } finally {
      setBusy(false);
    }
  };

  const sorted = useMemo(() => sortRows(sessions, compareSessions), [sessions, sorting]);
  const table = useTable({ columns, data: sorted });

  return (
    <div className="stack">
      <Text variant="header-2">Аккаунт</Text>
      {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
      {message && <Alert theme="success" message={message} onClose={() => setMessage('')} />}
      <Card view="filled" style={{ padding: 16 }}>
        <form className="stack" onSubmit={changePassword}>
          <Text variant="header-1">Смена пароля</Text>
          <TextInput placeholder="Текущий пароль" type="password" value={current} onUpdate={setCurrent} />
          <TextInput placeholder="Новый пароль (не короче 12)" type="password" value={next} onUpdate={setNext} />
          <div>
            <Button view="action" type="submit" loading={busy} disabled={!current || next.length < 12}>
              Сменить
            </Button>
          </div>
        </form>
      </Card>
      <Card view="filled" style={{ padding: 16 }}>
        <div className="stack">
          <Text variant="header-1">Двухфакторная аутентификация</Text>
          {!totp ? (
            <Text className="muted">Не удалось узнать состояние 2FA</Text>
          ) : !totp.available ? (
            <Text className="muted">
              Недоступна: на сервере не задан ключ TGADMIN_TOTP_KEY.
            </Text>
          ) : totp.enabled ? (
            <>
              <Text>
                Включена с {totp.createdAt ? formatTime(totp.createdAt) : '—'} · резервных кодов:{' '}
                {totp.recoveryLeft}
              </Text>
              <Text className="muted">
                При входе нужен код из приложения. Если телефон потерян, используйте резервный код.
              </Text>
              <div className="row">
                <TextInput
                  placeholder="Пароль"
                  type="password"
                  value={totpPassword}
                  onUpdate={setTotpPassword}
                />
                <TextInput placeholder="Код" value={totpCode} onUpdate={setTotpCode} />
                <Button
                  view="outlined"
                  loading={totpBusy}
                  onClick={() => {
                    setTotpBusy(true);
                    api
                      .totpDisable(totpPassword, totpCode)
                      .then(() => {
                        setMessage('2FA выключена');
                        setTotpPassword('');
                        setTotpCode('');
                        load();
                      })
                      .catch((err) =>
                        setError(err instanceof ApiError ? err.message : 'не удалось выключить'),
                      )
                      .finally(() => setTotpBusy(false));
                  }}
                >
                  Выключить
                </Button>
                <Button
                  view="outlined"
                  loading={totpBusy}
                  onClick={() => {
                    setTotpBusy(true);
                    api
                      .totpRecovery(totpPassword, totpCode)
                      .then((res) => {
                        setRecoveryCodes(res.recoveryCodes);
                        setTotpPassword('');
                        setTotpCode('');
                        load();
                      })
                      .catch((err) =>
                        setError(err instanceof ApiError ? err.message : 'не удалось перевыпустить'),
                      )
                      .finally(() => setTotpBusy(false));
                  }}
                >
                  Новые резервные коды
                </Button>
              </div>
            </>
          ) : (
            <>
              <Text className="muted">
                Защитите вход кодом из приложения: Aegis, Ente Auth, Google Authenticator, 1Password.
              </Text>
              {!setup ? (
                <div>
                  <Button
                    view="action"
                    loading={totpBusy}
                    onClick={() => {
                      setTotpBusy(true);
                      api
                        .totpSetup()
                        .then(setSetup)
                        .catch((err) =>
                          setError(err instanceof ApiError ? err.message : 'не удалось начать настройку'),
                        )
                        .finally(() => setTotpBusy(false));
                    }}
                  >
                    Включить 2FA
                  </Button>
                </div>
              ) : (
                <>
                  <div className="totp-setup">
                    <img src={setup.qr} alt="QR для аутентификатора" width={220} height={220} />
                    <div className="stack">
                      <Text>Секрет для ручного ввода:</Text>
                      <div className="link-box">{setup.secret}</div>
                      <Text className="muted">
                        Добавьте в приложение оба телефона, затем введите код ниже.
                      </Text>
                    </div>
                  </div>
                  <div className="row">
                    <TextInput placeholder="Код из приложения" value={totpCode} onUpdate={setTotpCode} />
                    <Button
                      view="action"
                      loading={totpBusy}
                      disabled={totpCode.trim().length !== 6}
                      onClick={() => {
                        setTotpBusy(true);
                        api
                          .totpEnable(totpCode)
                          .then((res) => {
                            setRecoveryCodes(res.recoveryCodes);
                            setSetup(null);
                            setTotpCode('');
                            setMessage('2FA включена. Сохраните резервные коды.');
                            load();
                          })
                          .catch((err) =>
                            setError(err instanceof ApiError ? err.message : 'не удалось включить'),
                          )
                          .finally(() => setTotpBusy(false));
                      }}
                    >
                      Подтвердить
                    </Button>
                    <Button view="flat" onClick={() => setSetup(null)}>
                      Отмена
                    </Button>
                  </div>
                </>
              )}
            </>
          )}
          {recoveryCodes.length > 0 && (
            <div className="stack">
              <Text variant="subheader-1">Резервные коды</Text>
              <Text className="muted">
                Каждый код работает один раз. Сохраните их отдельно от телефона.
              </Text>
              <pre className="recovery-codes">{recoveryCodes.join('\n')}</pre>
              <div className="row">
                <Button
                  view="outlined"
                  onClick={() => void navigator.clipboard.writeText(recoveryCodes.join('\n'))}
                >
                  Скопировать
                </Button>
                <Button
                  view="outlined"
                  onClick={() => {
                    const blob = new Blob([recoveryCodes.join('\n') + '\n'], {
                      type: 'text/plain;charset=utf-8',
                    });
                    const url = URL.createObjectURL(blob);
                    const link = document.createElement('a');
                    link.href = url;
                    link.download = 'telemt-admin-recovery-codes.txt';
                    link.click();
                    URL.revokeObjectURL(url);
                  }}
                >
                  Скачать
                </Button>
                <Button view="flat" onClick={() => setRecoveryCodes([])}>
                  Скрыть
                </Button>
              </div>
            </div>
          )}
        </div>
      </Card>
      <Card view="filled" style={{ padding: 8 }}>
        <Text variant="header-1" style={{ padding: 8 }}>
          Сессии
        </Text>
        {loading ? (
          <Loader size="m" />
        ) : isMobile ? (
          <div className="user-cards" style={{ padding: 8 }}>
            {sorted.map((item) => (
              <div className="user-card" key={item.id}>
                <div>{item.userAgent || 'устройство не определено'}</div>
                <div className="muted">
                  {item.ip} · {formatTime(item.createdAt)}
                </div>
                {item.current ? (
                  <Text color="secondary">текущая</Text>
                ) : (
                  <Button view="flat" onClick={() => void api.revoke(item.id).then(load)}>
                    Завершить
                  </Button>
                )}
              </div>
            ))}
            {sessions.length === 0 && <Text className="muted">Других сессий нет</Text>}
          </div>
        ) : (
          <Table table={table} emptyContent="Других сессий нет" />
        )}
      </Card>
    </div>
  );
}
