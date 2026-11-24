import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
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
import {
  ArrowLeft,
  ArrowRotateLeft,
  Ban,
  Copy,
  FileArrowDown,
  Pencil,
  Play,
  TrashBin,
} from '@gravity-ui/icons';
import { Table, useTable } from '@gravity-ui/table';
import type { ColumnDef } from '@gravity-ui/table/tanstack';
import {
  api,
  ApiError,
  type AuditItem,
  type IPHistoryItem,
  type Me,
  type ProxyUser,
  type TimelineDay,
  type TrafficDay,
} from '../api';
import { formatBytes, formatTime } from '../format';
import { useMedia } from '../useMedia';

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
  const [traffic, setTraffic] = useState<TrafficDay[]>([]);
  const [timeline, setTimeline] = useState<TimelineDay[]>([]);
  const [tab, setTab] = useState<'overview' | 'devices' | 'limits' | 'audit'>('overview');
  const [auditItems, setAuditItems] = useState<AuditItem[]>([]);
  const [trustedCountries, setTrustedCountries] = useState('');
  const [trustedIps, setTrustedIps] = useState('');
  const [trustedSaved, setTrustedSaved] = useState(false);

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
      setTrustedCountries((value.trustedCountries ?? []).join(', '));
      setTrustedIps((value.trustedIps ?? []).join(', '));
      api
        .userTraffic(name)
        .then(setTraffic)
        .catch(() => setTraffic([]));
      api
        .userTimeline(name)
        .then(setTimeline)
        .catch(() => setTimeline([]));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось загрузить пользователя');
    } finally {
      setLoading(false);
    }
  }, [name]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (tab !== 'audit' || me.user.role !== 'admin') return;
    api
      .audit({ target: name, limit: 50 })
      .then(setAuditItems)
      .catch(() => setAuditItems([]));
  }, [tab, name, me.user.role]);

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
        <div>
          <Button view="flat" onClick={() => navigate('/')}>
            <ArrowLeft width={16} height={16} /> К списку
          </Button>
        </div>
      </div>
    );
  }
  const isAdmin = me.user.role === 'admin';

  return (
    <div className="stack">
      <div className="page-head">
        <Button view="flat" aria-label="К списку" title="К списку" onClick={() => navigate('/')}>
          <ArrowLeft width={18} height={18} />
        </Button>
        <Text variant="header-2">{user.username}</Text>
        <Label theme={user.enabled ? 'success' : 'danger'} size="s">
          {user.enabled ? 'включён' : 'заблокирован'}
        </Label>
        {user.risk > 0 && (
          <Label theme={user.risk === 2 ? 'danger' : 'warning'} size="s">
            {user.risk === 2 ? 'высокий риск' : 'внимание'}
          </Label>
        )}
        {user.newCountries24h > 0 && (
          <Label theme="warning" size="s">
            новая страна
          </Label>
        )}
        {isAdmin && (
          <>
            <div style={{ flex: 1 }} />
            <div className="icon-actions">
              <Button
                view="flat"
                aria-label="Перевыпустить секрет"
                title="Перевыпустить секрет"
                onClick={() => setRotateOpen(true)}
              >
                <ArrowRotateLeft width={18} height={18} />
              </Button>
              <Button
                view="flat"
                aria-label={user.enabled ? 'Заблокировать' : 'Разблокировать'}
                title={user.enabled ? 'Заблокировать' : 'Разблокировать'}
                loading={busy}
                onClick={() =>
                  void run(() => (user.enabled ? api.disable(user.username) : api.enable(user.username)))
                }
              >
                {user.enabled ? <Ban width={18} height={18} /> : <Play width={18} height={18} />}
              </Button>
              <Button
                view="flat"
                aria-label="Переименовать"
                title="Переименовать"
                onClick={() => setRenameOpen(true)}
              >
                <Pencil width={18} height={18} />
              </Button>
              <Button
                view="flat-danger"
                aria-label="Удалить"
                title="Удалить"
                onClick={() => setDeleteOpen(true)}
              >
                <TrashBin width={18} height={18} />
              </Button>
            </div>
          </>
        )}
      </div>
      {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}

      <div className="tabs">
        <Button
          view={tab === 'overview' ? 'action' : 'flat'}
          onClick={() => setTab('overview')}
        >
          Обзор
        </Button>
        <Button view={tab === 'devices' ? 'action' : 'flat'} onClick={() => setTab('devices')}>
          Устройства
        </Button>
        {isAdmin && (
          <>
            <Button view={tab === 'limits' ? 'action' : 'flat'} onClick={() => setTab('limits')}>
              Лимиты
            </Button>
            <Button view={tab === 'audit' ? 'action' : 'flat'} onClick={() => setTab('audit')}>
              Аудит
            </Button>
          </>
        )}
      </div>

      {tab === 'overview' && (
        <>
      <Card view="filled" style={{ padding: 16 }}>
        <div className="stack">
          <div className="stat-grid">
            <Stat label="Соединения" value={user.currentConnections} />
            <Stat label="Активные IP" value={user.activeUniqueIps} />
            <Stat label="IP за 24 часа" value={user.ips24h} />
            <Stat label="IP за 7 дней" value={user.ips7d} />
            <Stat label="Новых IP за сутки" value={user.new24h} />
            <Stat label="Пик за сутки" value={user.maxToday} />
            <Stat
              label="Стран сейчас"
              value={(user.countries ?? []).length || '—'}
            />
            <Stat
              label="Иностранных IP"
              value={user.foreignIpsCurrent}
            />
            <Stat label="Иностранных за сутки" value={user.foreignIps24h} />
            <Stat label="Стран за 7 дней" value={user.countries7d} />
            <Stat label="Новых стран" value={user.newCountries24h} />
            <Stat label="Трафик всего" value={formatBytes(user.totalOctets)} />
            <Stat label="Трафик за сутки" value={formatBytes(user.trafficToday)} />
          </div>
          <Text className="muted">
            IP сейчас: {user.activeIps.join(', ') || '—'} · лимит IP: {user.maxUniqueIps ?? 'нет'} ·
            лимит соединений: {user.maxTcpConns ?? 'нет'}
          </Text>
          {(user.countries?.length ?? 0) > 0 && (
            <Text className="muted">Страны сейчас: {user.countries!.join(', ')}</Text>
          )}
          {user.newCountries24h > 0 && (
            <Text className="muted">
              За сутки появилась новая страна: проверьте устройства и при необходимости
              перевыпустите секрет.
            </Text>
          )}
          {user.note && <Text className="muted">Заметка: {user.note}</Text>}
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
                <Pencil width={16} height={16} /> Сохранить заметку
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
            <div className="link-grid">
              <div className="stack">
                <div className="link-box">{user.link}</div>
                <Alert theme="warning" message="Ссылка равна паролю: не публикуйте её." />
                <div className="row">
                  <Button
                    view="outlined"
                    onClick={() =>
                      void navigator.clipboard.writeText(user.link!).then(() => setCopied(true))
                    }
                  >
                    <Copy width={16} height={16} /> Скопировать
                  </Button>
                  {copied && <Text color="positive">скопировано</Text>}
                </div>
              </div>
              <a href={api.qrUrl(user.username)} download={`qr-${user.username}.png`}>
                <img className="qr" src={api.qrUrl(user.username)} alt="QR" width={256} height={256} />
              </a>
            </div>
          </div>
        </Card>
      )}

      <Card view="filled" style={{ padding: 16 }}>
        <div className="stack">
          <div className="card-head">
            <Text variant="header-1">Трафик по дням</Text>
            <div style={{ flex: 1 }} />
            <Text className="muted">30 дней</Text>
          </div>
          <TrafficChart items={traffic} />
        </div>
      </Card>

        </>
      )}

      {tab === 'devices' && (
        <>
      <Card view="filled" style={{ padding: 16 }}>
        <div className="stack">
          <div className="card-head">
            <Text variant="header-1">Устройства</Text>
            <div style={{ flex: 1 }} />
            {isAdmin && (
              <div className="icon-actions">
                <Button
                  view="flat"
                  aria-label="Экспорт истории в CSV"
                  title="Экспорт истории в CSV"
                  onClick={() => window.location.assign(api.historyCSVUrl(user.username))}
                >
                  <FileArrowDown width={18} height={18} />
                </Button>
                <Button
                  view="flat"
                  aria-label="Удалить историю"
                  title="Удалить историю"
                  onClick={() => setHistoryOpen(true)}
                >
                  <TrashBin width={18} height={18} />
                </Button>
              </div>
            )}
          </div>
          {user.risk > 0 && (
            <Alert
              theme={user.risk === 2 ? 'danger' : 'warning'}
              message="Столько одновременных адресов не похоже на обычное число устройств. Проверьте, не передан ли ключ."
            />
          )}
          <Text className="muted">
            Адреса за 24 часа: {user.ips24h}, за 7 дней: {user.ips7d}, новых: {user.new24h}, пик: {user.maxToday}
          </Text>
          {isAdmin && user.ipHistory && user.ipHistory.length > 0 && <IPHistoryTable items={user.ipHistory} />}
        </div>
      </Card>

          <Card view="filled" style={{ padding: 16 }}>
            <div className="stack">
              <Text variant="header-1">Активность по дням</Text>
              <Timeline items={timeline} />
            </div>
          </Card>

          {isAdmin && (
            <Card view="filled" style={{ padding: 16 }}>
              <div className="stack">
                <Text variant="header-1">Доверенные страны и адреса</Text>
                <Text className="muted">
                  Такие адреса остаются в истории, но не поднимают риск и не дают алертов.
                </Text>
                <label className="field">
                  <span>Страны, коды ISO через запятую</span>
                  <TextInput
                    placeholder="например, TR, GE"
                    value={trustedCountries}
                    onUpdate={(value) => {
                      setTrustedCountries(value);
                      setTrustedSaved(false);
                    }}
                  />
                </label>
                <label className="field">
                  <span>Адреса через запятую</span>
                  <TextInput
                    placeholder="например, 203.0.113.10"
                    value={trustedIps}
                    onUpdate={(value) => {
                      setTrustedIps(value);
                      setTrustedSaved(false);
                    }}
                  />
                </label>
                <div className="row">
                  <Button
                    view="outlined"
                    onClick={() =>
                      void api
                        .setTrusted(user.username, {
                          countries: trustedCountries
                            .split(/[\s,]+/)
                            .map((v) => v.trim().toUpperCase())
                            .filter(Boolean),
                          ips: trustedIps
                            .split(/[\s,]+/)
                            .map((v) => v.trim())
                            .filter(Boolean),
                        })
                        .then(() => {
                          setTrustedSaved(true);
                          void load();
                        })
                    }
                  >
                    <Pencil width={16} height={16} /> Сохранить доверенные
                  </Button>
                  {trustedSaved && <Text color="positive">сохранено</Text>}
                </div>
              </div>
            </Card>
          )}
        </>
      )}

      {tab === 'limits' && isAdmin && (
        <>
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
              <div className="form-grid">
                <section className="form-section">
                  <Text variant="subheader-1">Адреса и соединения</Text>
                  <label className="field">
                    <span>Максимум уникальных IP</span>
                    <TextInput
                      placeholder="например, 3"
                      value={maxIps}
                      onUpdate={setMaxIps}
                      disabled={clearIps}
                    />
                  </label>
                  <label className="check">
                    <input type="checkbox" checked={clearIps} onChange={(e) => setClearIps(e.target.checked)} />
                    снять лимит IP
                  </label>
                  <label className="field">
                    <span>Максимум соединений</span>
                    <TextInput
                      placeholder="например, 10"
                      value={maxConns}
                      onUpdate={setMaxConns}
                      disabled={clearConns}
                    />
                  </label>
                  <label className="check">
                    <input type="checkbox" checked={clearConns} onChange={(e) => setClearConns(e.target.checked)} />
                    снять лимит соединений
                  </label>
                </section>

                <section className="form-section">
                  <Text variant="subheader-1">Квота трафика</Text>
                  <label className="field">
                    <span>Квота, ГиБ</span>
                    <TextInput
                      placeholder="пусто — без квоты"
                      value={quotaGB}
                      onUpdate={setQuotaGB}
                      disabled={clearQuota}
                    />
                  </label>
                  <label className="field">
                    <span>Период сброса</span>
                    <select value={period} onChange={(e) => setPeriod(e.target.value)} disabled={clearQuota}>
                      <option value="">без сброса</option>
                      <option value="daily">сутки</option>
                      <option value="weekly">неделя</option>
                      <option value="monthly">месяц</option>
                    </select>
                  </label>
                  <label className="check">
                    <input type="checkbox" checked={clearQuota} onChange={(e) => setClearQuota(e.target.checked)} />
                    снять квоту
                  </label>
                </section>

                <section className="form-section">
                  <Text variant="subheader-1">Скорость</Text>
                  <label className="field">
                    <span>Вверх, Кбит/с</span>
                    <TextInput value={rateUp} onUpdate={setRateUp} disabled={clearRates} />
                  </label>
                  <label className="field">
                    <span>Вниз, Кбит/с</span>
                    <TextInput value={rateDown} onUpdate={setRateDown} disabled={clearRates} />
                  </label>
                  <label className="check">
                    <input type="checkbox" checked={clearRates} onChange={(e) => setClearRates(e.target.checked)} />
                    снять лимиты скорости
                  </label>
                </section>

                <section className="form-section">
                  <Text variant="subheader-1">Срок действия</Text>
                  <label className="field">
                    <span>Действует до</span>
                    <input
                      type="datetime-local"
                      value={expiration}
                      disabled={clearExpiration}
                      onChange={(e) => setExpiration(e.target.value)}
                    />
                  </label>
                  <label className="check">
                    <input
                      type="checkbox"
                      checked={clearExpiration}
                      onChange={(e) => setClearExpiration(e.target.checked)}
                    />
                    без срока
                  </label>
                </section>

                <section className="form-section span-2">
                  <Text variant="subheader-1">Запрещённые сети (source_deny)</Text>
                  <textarea
                    rows={3}
                    value={sourceDeny}
                    disabled={clearSource}
                    onChange={(e) => setSourceDeny(e.target.value)}
                    placeholder="по одному CIDR на строку, например 203.0.113.0/24"
                  />
                  <label className="check">
                    <input type="checkbox" checked={clearSource} onChange={(e) => setClearSource(e.target.checked)} />
                    очистить список
                  </label>
                </section>
              </div>

              <div className="form-actions">
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

      {tab === 'audit' && isAdmin && (
        <Card view="filled" style={{ padding: 16 }}>
          <div className="stack">
            <div className="card-head">
              <Text variant="header-1">Аудит клиента</Text>
              <div style={{ flex: 1 }} />
              <a href={api.auditCSVUrl()} download="audit.csv" className="muted">
                весь аудит
              </a>
            </div>
            {auditItems.length === 0 ? (
              <Text className="muted">Событий нет</Text>
            ) : (
              auditItems.map((event) => (
                <div className="timeline-row" key={`${event.at}-${event.action}`}>
                  <span className="timeline-date">{formatTime(event.at)}</span>
                  <div className="timeline-events">
                    <Text>{event.action}</Text>
                    <span className="muted">{event.actor}</span>
                    <span className="muted">{event.result}</span>
                  </div>
                </div>
              ))
            )}
          </div>
        </Card>
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

function Timeline({ items }: { items: TimelineDay[] }) {
  if (items.length === 0) {
    return <Text className="muted">Событий пока нет</Text>;
  }
  return (
    <div className="timeline">
      {[...items].reverse().map((day) => (
        <div className="timeline-row" key={day.day}>
          <span className="timeline-date">
            {new Date(`${day.day}T00:00:00Z`).toLocaleDateString('ru-RU')}
          </span>
          <div className="timeline-events">
            {day.newIps > 0 && (
              <Label size="s" theme="info">
                +{day.newIps} IP
              </Label>
            )}
            {(day.newCountries ?? []).map((country) => (
              <Label size="s" theme="warning" key={country.code}>
                {country.name}
              </Label>
            ))}
            {day.maxIps > 0 && <span className="muted">пик {day.maxIps}</span>}
            {day.bytes > 0 && <span className="muted">{formatBytes(day.bytes)}</span>}
          </div>
        </div>
      ))}
    </div>
  );
}

function TrafficChart({ items }: { items: TrafficDay[] }) {
  if (items.length === 0) {
    return <Text className="muted">Данных пока нет: первый срез появится через минуту</Text>;
  }
  const max = Math.max(...items.map((item) => item.bytes), 1);
  return (
    <div className="traffic-chart">
      {items.map((item, index) => {
        const height = Math.max(2, Math.round((item.bytes / max) * 100));
        const showLabel = index === 0 || index === items.length - 1 || index % 5 === 0;
        return (
          <div
            className="traffic-col"
            key={item.day}
            title={`${item.day}: ${formatBytes(item.bytes)}`}
          >
            <div className="traffic-bar" style={{ height: `${height}%` }} />
            <span className="traffic-label">{showLabel ? item.day.slice(8) : ''}</span>
          </div>
        );
      })}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="stat-tile">
      <span className="stat-label">{label}</span>
      <span className="stat-value">{value}</span>
    </div>
  );
}

function IPHistoryTable({ items }: { items: IPHistoryItem[] }) {
  const isMobile = useMedia('(max-width: 760px)');
  const columns = useMemo<ColumnDef<IPHistoryItem>[]>(() => [
    { id: 'ip', header: 'IP', cell: ({ row }) => row.original.ip },
    {
      id: 'country',
      header: 'Страна',
      cell: ({ row }) => {
        const item = row.original;
        if (item.internal) return <Label size="s" theme="unknown">локальный</Label>;
        if (!item.country) return '—';
        return (
          <span title={item.country}>
            {item.countryName ?? item.country} <span className="muted">{item.country}</span>
          </span>
        );
      },
    },
    {
      id: 'asn',
      header: 'Оператор',
      cell: ({ row }) => {
        const item = row.original;
        if (!item.asn && !item.asnOrg) return '—';
        return (
          <span title={item.asn ? `AS${item.asn}` : undefined}>
            {item.asnOrg ?? '—'}
            {item.asn ? <span className="muted"> AS{item.asn}</span> : null}
          </span>
        );
      },
    },
    ...(isMobile
      ? [
          {
            id: 'last',
            header: 'Последний раз',
            cell: ({ row }: { row: { original: IPHistoryItem } }) =>
              new Date(row.original.lastSeen).toLocaleString('ru-RU'),
          },
        ]
      : [
          { id: 'first', header: 'Впервые', cell: ({ row }: { row: { original: IPHistoryItem } }) => new Date(row.original.firstSeen).toLocaleString('ru-RU') },
          { id: 'last', header: 'Последний раз', cell: ({ row }: { row: { original: IPHistoryItem } }) => new Date(row.original.lastSeen).toLocaleString('ru-RU') },
          { id: 'count', header: 'Наблюдений', cell: ({ row }: { row: { original: IPHistoryItem } }) => row.original.count },
        ]),
  ], [isMobile]);
  const table = useTable({ columns, data: items });
  return (
    <div className="table-scroll">
      <Table table={table} />
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
