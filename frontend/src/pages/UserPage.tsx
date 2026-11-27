import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Dialog,
  Label,
  Loader,
  Select,
  Text,
  TextArea,
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
  type LimitTemplate,
  type Me,
  type ProxyUser,
  type QuotaResetView,
  type RiskIP,
  type RiskReport,
  type TimelineDay,
  type TrafficDay,
} from '../api';
import { formatBytes, formatTime } from '../format';
import { useTableSort } from '../tableSort';
import { useMedia } from '../useMedia';
import { DateFormField } from '../components/DateFormField';

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
  const [tab, setTab] = useState<'overview' | 'devices' | 'risk' | 'limits' | 'audit'>('overview');
  const [auditItems, setAuditItems] = useState<AuditItem[]>([]);
  const [trustedCountries, setTrustedCountries] = useState('');
  const [trustedIps, setTrustedIps] = useState('');
  const [trustedSaved, setTrustedSaved] = useState(false);
  const [riskWarn, setRiskWarn] = useState('');
  const [riskHigh, setRiskHigh] = useState('');
  const [riskDistinct, setRiskDistinct] = useState('');
  const [riskNewMode, setRiskNewMode] = useState<'inherit' | 'yes' | 'no'>('inherit');
  const [riskSaved, setRiskSaved] = useState(false);
  const [templates, setTemplates] = useState<LimitTemplate[]>([]);
  const [templateName, setTemplateName] = useState('');
  const [saveTemplateOpen, setSaveTemplateOpen] = useState(false);
  const [saveTemplateName, setSaveTemplateName] = useState('');
  const [saveTemplateError, setSaveTemplateError] = useState('');
  const [templateBusy, setTemplateBusy] = useState(false);
  const [riskReport, setRiskReport] = useState<RiskReport | null>(null);
  const [riskDays, setRiskDays] = useState(7);
  const [riskLoading, setRiskLoading] = useState(false);
  const [riskDay, setRiskDay] = useState('');
  const [chartDays, setChartDays] = useState(30);
  const [quotaReset, setQuotaReset] = useState<QuotaResetView | null>(null);
  const [quotaMode, setQuotaMode] = useState('inherit');
  const [quotaDays, setQuotaDays] = useState('');
  const [quotaTime, setQuotaTime] = useState('');
  const [quotaSaved, setQuotaSaved] = useState(false);
  const [quotaBusy, setQuotaBusy] = useState(false);

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
      setRiskWarn(value.riskThresholds?.warnIps?.toString() ?? '');
      setRiskHigh(value.riskThresholds?.highIps?.toString() ?? '');
      setRiskDistinct(value.riskThresholds?.distinct7d?.toString() ?? '');
      setRiskNewMode(
        value.riskThresholds?.newCountries == null
          ? 'inherit'
          : value.riskThresholds.newCountries
            ? 'yes'
            : 'no',
      );
      api
        .quotaReset(name)
        .then((value) => {
          setQuotaReset(value);
          setQuotaMode(value.mode);
          setQuotaDays(value.days?.toString() ?? '');
          setQuotaTime(value.time ?? '');
        })
        .catch(() => setQuotaReset(null));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось загрузить пользователя');
    } finally {
      setLoading(false);
    }
  }, [name]);

  useEffect(() => {
    api
      .meta()
      .then((meta) => {
        if (meta.defaultChartDays && meta.defaultChartDays > 0) {
          setChartDays(meta.defaultChartDays);
        }
      })
      .catch(() => {});
  }, []);

  useEffect(() => {
    setRiskSaved(false);
    void load();
  }, [load]);

  // Графики зависят только от периода: смена периода не перезагружает
  // карточку и не перетирает несохранённые поля формы.
  useEffect(() => {
    let cancelled = false;
    api
      .userTraffic(name, chartDays)
      .then((value) => {
        if (!cancelled) setTraffic(value);
      })
      .catch(() => {
        if (!cancelled) setTraffic([]);
      });
    api
      .userTimeline(name, chartDays)
      .then((value) => {
        if (!cancelled) setTimeline(value);
      })
      .catch(() => {
        if (!cancelled) setTimeline([]);
      });
    return () => {
      cancelled = true;
    };
  }, [name, chartDays]);

  useEffect(() => {
    api
      .templates()
      .then((data) => {
        setTemplates(data.templates);
        setTemplateName(data.default || data.templates[0]?.name || '');
      })
      .catch(() => setTemplates([]));
  }, []);

  useEffect(() => {
    if (tab !== 'risk' || me.user.role !== 'admin') return;
    let cancelled = false;
    setRiskLoading(true);
    api
      .riskReport(name, riskDays)
      .then((data) => {
        if (cancelled) return;
        setRiskReport(data);
        setRiskDay((prev) =>
          data.days.some((day) => day.day === prev) ? prev : (data.days[0]?.day ?? ''),
        );
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof ApiError ? err.message : 'не удалось загрузить отчёт');
      })
      .finally(() => {
        if (!cancelled) setRiskLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [tab, name, riskDays, me.user.role]);

  useEffect(() => {
    if (tab !== 'audit' || me.user.role !== 'admin') return;
    api
      .audit({ target: name, limit: 50 })
      .then(setAuditItems)
      .catch(() => setAuditItems([]));
  }, [tab, name, me.user.role]);

  const applyTemplate = () => {
    const template = templates.find((item) => item.name === templateName);
    if (!template) return;
    setMaxIps(template.maxIps?.toString() ?? '');
    setMaxConns(template.maxTcpConns?.toString() ?? '');
    setQuotaGB(template.dataQuotaBytes ? (template.dataQuotaBytes / 1024 ** 3).toFixed(2) : '');
    setPeriod(template.quotaPeriod ?? '');
    setRateUp(template.rateLimitUpBps ? (template.rateLimitUpBps / 1000).toString() : '');
    setRateDown(template.rateLimitDownBps ? (template.rateLimitDownBps / 1000).toString() : '');
    setExpiration(template.expirationDays ? daysFromNow(template.expirationDays) : '');
    setSourceDeny((template.sourceDeny ?? []).join('\n'));
    setRiskWarn(template.riskWarnIps?.toString() ?? '');
    setRiskHigh(template.riskHighIps?.toString() ?? '');
    setRiskDistinct(template.riskDistinct7d?.toString() ?? '');
    setRiskNewMode(
      template.riskNewCountries == null
        ? 'inherit'
        : template.riskNewCountries
          ? 'yes'
          : 'no',
    );
  };

  const saveQuotaSchedule = async () => {
    if (!user) return;
    setQuotaBusy(true);
    setError('');
    setQuotaSaved(false);
    try {
      const value = await api.setQuotaReset(user.username, {
        mode: quotaMode,
        days: quotaMode === 'custom' ? Number(quotaDays) : undefined,
        time: quotaMode === 'custom' ? quotaTime : undefined,
      });
      setQuotaReset(value);
      setQuotaMode(value.mode);
      setQuotaDays(value.days?.toString() ?? '');
      setQuotaTime(value.time ?? '');
      setQuotaSaved(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось сохранить расписание');
    } finally {
      setQuotaBusy(false);
    }
  };

  const saveAsTemplate = async () => {
    const name = saveTemplateName.trim();
    if (!name) return;
    setTemplateBusy(true);
    setSaveTemplateError('');
    try {
      await api.saveTemplate({
        name,
        maxIps: maxIps ? Number(maxIps) : undefined,
        maxTcpConns: maxConns ? Number(maxConns) : undefined,
        dataQuotaBytes: quotaGB ? Math.round(Number(quotaGB) * 1024 ** 3) : undefined,
        quotaPeriod: period || undefined,
        rateLimitUpBps: rateUp ? Math.round(Number(rateUp) * 1000) : undefined,
        rateLimitDownBps: rateDown ? Math.round(Number(rateDown) * 1000) : undefined,
        expirationDays: expiration
          ? Math.max(0, Math.ceil((new Date(expiration).getTime() - Date.now()) / 86_400_000))
          : undefined,
        sourceDeny: sourceDeny.split(/[\n,]+/).map((v) => v.trim()).filter(Boolean),
        riskWarnIps: riskWarn ? Number(riskWarn) : undefined,
        riskHighIps: riskHigh ? Number(riskHigh) : undefined,
        riskDistinct7d: riskDistinct ? Number(riskDistinct) : undefined,
        riskNewCountries: riskNewMode === 'inherit' ? undefined : riskNewMode === 'yes',
      });
      const data = await api.templates();
      setTemplates(data.templates);
      setTemplateName(name);
      setSaveTemplateOpen(false);
      setSaveTemplateName('');
    } catch (err) {
      setSaveTemplateError(err instanceof ApiError ? err.message : 'не удалось сохранить шаблон');
    } finally {
      setTemplateBusy(false);
    }
  };

  const trustRisk = (payload: { ip?: string; country?: string }) => {
    if (!user) return;
    void run(async () => {
      const current = await api.trusted(user.username);
      const countries = Array.from(
        new Set([...current.countries, ...(payload.country ? [payload.country] : [])]),
      );
      const ips = Array.from(new Set([...current.ips, ...(payload.ip ? [payload.ip] : [])]));
      await api.setTrusted(user.username, { countries, ips });
      setRiskReport(await api.riskReport(user.username, riskDays));
    });
  };

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
  const selectedRiskDay =
    riskReport?.days.find((day) => day.day === riskDay) ?? riskReport?.days[0];

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
        {isAdmin && user.risk > 0 && (
          <Button view="flat" onClick={() => setTab('risk')}>
            Отчёт по риску
          </Button>
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
            <Button view={tab === 'risk' ? 'action' : 'flat'} onClick={() => setTab('risk')}>
              Риск
            </Button>
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
            <Text className="muted">{chartDays} дней</Text>
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

      {tab === 'risk' && isAdmin && (
        <Card view="filled" style={{ padding: 16 }}>
          <div className="stack">
            <div className="card-head">
              <Text variant="header-1">Отчёт по риску</Text>
              <div style={{ flex: 1 }} />
              <a href={api.riskCSVUrl(user.username, riskDays)} download={`risk-${user.username}.csv`}>
                Экспорт CSV
              </a>
            </div>

            {riskLoading && <Loader size="m" />}
            {!riskLoading && !riskReport && <Text className="muted">Нет данных</Text>}

            {riskReport && (
              <>
                {riskReport.reasons.length === 0 ? (
                  <Alert theme="success" message="Риск в пределах действующих порогов." />
                ) : (
                  riskReport.reasons.map((reason) => (
                    <Alert
                      key={reason.code}
                      theme={reason.severity === 2 ? 'danger' : 'warning'}
                      title={reason.severity === 2 ? 'Высокий риск' : 'Предупреждение'}
                      message={reason.message}
                    />
                  ))
                )}

                <Text className="muted">
                  Пороги {riskReport.custom ? 'свои' : 'глобальные'}: предупреждение{' '}
                  {riskReport.thresholds.warnIps}, высокий {riskReport.thresholds.highIps}, адресов за
                  7 дней {riskReport.thresholds.distinct7d}, новые страны{' '}
                  {riskReport.thresholds.newCountries ? 'учитываются' : 'не учитываются'}.
                </Text>
                {(riskReport.current.countries?.length ?? 0) > 0 && (
                  <Text className="muted">
                    Страны сейчас:{' '}
                    {riskReport.current.countries?.map((country) => country.name || country.code).join(', ')}
                  </Text>
                )}

                <div className="stat-grid">
                  <Stat label="Активных адресов" value={riskReport.current.current} />
                  <Stat label="Доверенных" value={riskReport.current.trustedActive} />
                  <Stat label="Адресов за 24 часа" value={riskReport.current.distinct24h} />
                  <Stat label="Адресов за 7 дней" value={riskReport.current.distinct7d} />
                  <Stat label="Пик за сутки" value={riskReport.current.maxToday} />
                  <Stat label="Новых за сутки" value={riskReport.current.newIps24h} />
                  <Stat label="Новых стран" value={riskReport.current.newCountries} />
                  <Stat label="Иностранных за сутки" value={riskReport.current.foreignIps24h} />
                </div>
                <Text className="muted">
                  По данным прокси: один адрес может скрывать несколько устройств за NAT, а одно
                  устройство может менять адрес.
                </Text>

                <div className="row">
                  <label className="field field-inline">
                    <span>Период</span>
                    <Select
                      aria-label="Период"
                      value={[String(riskDays)]}
                      options={[
                        { value: '7', content: '7 дней' },
                        { value: '30', content: '30 дней' },
                      ]}
                      onUpdate={([value]) => setRiskDays(Number(value))}
                      width={140}
                    />
                  </label>
                  <div style={{ flex: 1 }} />
                  <Button view="outlined" onClick={() => setRotateOpen(true)}>
                    Перевыпустить секрет
                  </Button>
                </div>

                {riskReport.days.length === 0 ? (
                  <Text className="muted">Истории адресов пока нет.</Text>
                ) : (
                  <>
                    <Text variant="subheader-1">Дни</Text>
                    <div className="risk-days">
                      {riskReport.days.map((day) => (
                        <Button
                          key={day.day}
                          view="flat"
                          className="risk-day"
                          selected={riskDay === day.day}
                          onClick={() => setRiskDay(day.day)}
                        >
                          <Text variant="subheader-1">{formatRiskDay(day.day)}</Text>
                          <span className="muted">
                            {day.distinct} адресов · пик {day.max}
                          </span>
                          <span className="muted">
                            +{day.newIps} новых · {formatBytes(day.traffic)}
                          </span>
                          {(day.newCountries?.length ?? 0) > 0 && (
                            <span className="muted">
                              новые страны: {day.newCountries?.map((country) => country.code).join(', ')}
                            </span>
                          )}
                        </Button>
                      ))}
                    </div>

                    {selectedRiskDay && (
                      <div className="stack">
                        <Text variant="subheader-1">{formatRiskDay(selectedRiskDay.day)}</Text>
                        {selectedRiskDay.ips.length === 0 ? (
                          <Text className="muted">За этот день адресов нет.</Text>
                        ) : (
                          <RiskDayTable
                            items={selectedRiskDay.ips}
                            onTrustIP={(ip) => trustRisk({ ip })}
                            onTrustCountry={(code) => trustRisk({ country: code })}
                          />
                        )}
                      </div>
                    )}
                  </>
                )}
              </>
            )}
          </div>
        </Card>
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
              {templates.length > 0 && (
                <div className="row">
                  <label className="field field-inline">
                    <span>Шаблон</span>
                    <Select
                      aria-label="Шаблон"
                      value={templateName ? [templateName] : []}
                      options={templates.map((template) => ({
                        value: template.name,
                        content: template.name,
                      }))}
                      onUpdate={([value]) => setTemplateName(value ?? '')}
                      width={180}
                    />
                  </label>
                  <Button view="outlined" type="button" onClick={applyTemplate}>
                    Заполнить из шаблона
                  </Button>
                  <Button
                    view="flat"
                    type="button"
                    onClick={() => {
                      setSaveTemplateName('');
                      setSaveTemplateError('');
                      setSaveTemplateOpen(true);
                    }}
                  >
                    Сохранить как шаблон
                  </Button>
                </div>
              )}
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
                    <Checkbox
                      controlProps={{ 'aria-label': 'снять лимит IP' }}
                      checked={clearIps}
                      onUpdate={setClearIps}
                      content="снять лимит IP"
                    />
                  <label className="field">
                    <span>Максимум соединений</span>
                    <TextInput
                      placeholder="например, 10"
                      value={maxConns}
                      onUpdate={setMaxConns}
                      disabled={clearConns}
                    />
                  </label>
                    <Checkbox
                      controlProps={{ 'aria-label': 'снять лимит соединений' }}
                      checked={clearConns}
                      onUpdate={setClearConns}
                      content="снять лимит соединений"
                    />
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
                    <Select
                      aria-label="Период квоты"
                      value={period ? [period] : []}
                      options={[
                        { value: '', content: 'без сброса' },
                        { value: 'daily', content: 'сутки' },
                        { value: 'weekly', content: 'неделя' },
                        { value: 'monthly', content: 'месяц' },
                      ]}
                      onUpdate={([value]) => setPeriod(value ?? '')}
                      disabled={clearQuota}
                      placeholder="без сброса"
                      width="max"
                    />
                  </label>
                    <Checkbox
                      controlProps={{ 'aria-label': 'снять квоту' }}
                      checked={clearQuota}
                      onUpdate={setClearQuota}
                      content="снять квоту"
                    />
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
                    <Checkbox
                      controlProps={{ 'aria-label': 'снять лимиты скорости' }}
                      checked={clearRates}
                      onUpdate={setClearRates}
                      content="снять лимиты скорости"
                    />
                </section>

                <section className="form-section">
                  <Text variant="subheader-1">Срок действия</Text>
                  <DateFormField
                    label="Действует до"
                    value={expiration}
                    withTime
                    disabled={clearExpiration}
                    onUpdate={setExpiration}
                  />
                  <Checkbox
                    controlProps={{ 'aria-label': 'без срока' }}
                    checked={clearExpiration}
                    onUpdate={setClearExpiration}
                    content="без срока"
                  />
                </section>

                <section className="form-section span-2">
                  <Text variant="subheader-1">Запрещённые сети (source_deny)</Text>
                  <TextArea
                    rows={3}
                    value={sourceDeny}
                    disabled={clearSource}
                    onUpdate={setSourceDeny}
                    placeholder="по одному CIDR на строку, например 203.0.113.0/24"
                  />
                  <Checkbox
                    controlProps={{ 'aria-label': 'очистить список' }}
                    checked={clearSource}
                    onUpdate={setClearSource}
                    content="очистить список"
                  />
                </section>
              </div>

              <div className="form-actions">
                <Button view="action" type="submit" loading={busy}>
                  Сохранить
                </Button>
                <Button
                  view="outlined"
                  onClick={() =>
                    void run(async () => {
                      await api.resetQuota(user.username);
                      setQuotaReset(await api.quotaReset(user.username));
                    })
                  }
                >
                  Сбросить квоту
                </Button>
              </div>
            </form>
          </Card>

          <Card view="filled" style={{ padding: 16 }}>
            <div className="stack">
              <Text variant="header-1">Автосброс квоты</Text>
              <Text className="muted">
                Обнуляет только расход квоты в telemt. Лимиты, ссылки, история и трафик
                сохраняются. {quotaReset?.enabled
                  ? `Следующий сброс: ${quotaReset.next ? formatTime(quotaReset.next) : 'после первого запуска'}.`
                  : 'Сейчас выключен.'}
              </Text>
              <div className="form-grid">
                <label className="field">
                  <span>Режим</span>
                  <Select
                    aria-label="Режим"
                    value={[quotaMode]}
                    options={[
                      { value: 'inherit', content: 'как у всех' },
                      { value: 'off', content: 'не сбрасывать' },
                      { value: 'custom', content: 'своё расписание' },
                    ]}
                    onUpdate={([value]) => setQuotaMode(value ?? 'inherit')}
                    width="max"
                  />
                </label>
                <label className="field">
                  <span>Интервал, дней</span>
                  <TextInput
                    placeholder="например, 30"
                    value={quotaDays}
                    onUpdate={setQuotaDays}
                    disabled={quotaMode !== 'custom'}
                  />
                </label>
                <label className="field">
                  <span>Время</span>
                  <TextInput
                    placeholder="04:00"
                    value={quotaTime}
                    onUpdate={setQuotaTime}
                    disabled={quotaMode !== 'custom'}
                  />
                </label>
              </div>
              <div className="form-actions">
                <Button
                  view="action"
                  loading={quotaBusy}
                  onClick={() => void saveQuotaSchedule()}
                >
                  Сохранить расписание
                </Button>
                {quotaSaved && <Text color="positive">сохранено</Text>}
              </div>
            </div>
          </Card>

          <Card view="filled" style={{ padding: 16 }}>
            <form
              className="stack"
              onSubmit={(event) => {
                event.preventDefault();
                void run(async () => {
                  await api.setRisk(user.username, {
                    warnIps: riskWarn ? Number(riskWarn) : null,
                    highIps: riskHigh ? Number(riskHigh) : null,
                    distinct7d: riskDistinct ? Number(riskDistinct) : null,
                    newCountries: riskNewMode === 'inherit' ? null : riskNewMode === 'yes',
                  });
                  setRiskSaved(true);
                });
              }}
            >
              <Text variant="header-1">Пороги риска</Text>
              <Text className="muted">
                Пустое поле — общее значение: предупреждение {user.riskDefaults?.warnIps ?? '—'}, высокий риск{' '}
                {user.riskDefaults?.highIps ?? '—'}, адресов за 7 дней {user.riskDefaults?.distinct7d ?? '—'}.
              </Text>
              <div className="form-grid">
                <section className="form-section">
                  <Text variant="subheader-1">Активные адреса</Text>
                  <label className="field">
                    <span>Предупреждение, больше IP</span>
                    <TextInput
                      placeholder={`например, ${user.riskDefaults?.warnIps ?? 3}`}
                      value={riskWarn}
                      onUpdate={setRiskWarn}
                    />
                  </label>
                  <label className="field">
                    <span>Высокий риск, больше IP</span>
                    <TextInput
                      placeholder={`например, ${user.riskDefaults?.highIps ?? 5}`}
                      value={riskHigh}
                      onUpdate={setRiskHigh}
                    />
                  </label>
                  <label className="field">
                    <span>Адресов за 7 дней</span>
                    <TextInput
                      placeholder={`например, ${user.riskDefaults?.distinct7d ?? 15}`}
                      value={riskDistinct}
                      onUpdate={setRiskDistinct}
                    />
                  </label>
                </section>
                <section className="form-section">
                  <Text variant="subheader-1">Новые страны</Text>
                  <label className="field">
                    <span>Реакция на новую страну</span>
                    <Select
                      aria-label="Реакция на новую страну"
                      value={[riskNewMode]}
                      options={[
                        { value: 'inherit', content: 'как у всех' },
                        { value: 'yes', content: 'считать риском' },
                        { value: 'no', content: 'не считать' },
                      ]}
                      onUpdate={([value]) =>
                        setRiskNewMode((value ?? 'inherit') as 'inherit' | 'yes' | 'no')
                      }
                      width="max"
                    />
                  </label>
                </section>
              </div>
              <div className="form-actions">
                <Button view="action" type="submit" loading={busy}>
                  Сохранить пороги
                </Button>
                <Button
                  view="outlined"
                  onClick={() =>
                    void run(async () => {
                      setRiskWarn('');
                      setRiskHigh('');
                      setRiskDistinct('');
                      setRiskNewMode('inherit');
                      await api.setRisk(user.username, {
                        warnIps: null,
                        highIps: null,
                        distinct7d: null,
                        newCountries: null,
                      });
                      setRiskSaved(true);
                    })
                  }
                >
                  Сбросить к общим
                </Button>
                {riskSaved && <Text color="positive">сохранено</Text>}
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

      <Dialog open={saveTemplateOpen} onOpenChange={setSaveTemplateOpen} maxWidth="s">
        <Dialog.Header caption="Сохранить как шаблон" />
        <Dialog.Body>
          <div className="stack">
            {saveTemplateError && (
              <Alert theme="danger" message={saveTemplateError} onClose={() => setSaveTemplateError('')} />
            )}
            <Text className="muted">
              В шаблон попадут текущие лимиты и пороги риска из формы.
            </Text>
            <TextInput
              placeholder="Имя шаблона"
              value={saveTemplateName}
              onUpdate={setSaveTemplateName}
            />
          </div>
        </Dialog.Body>
        <Dialog.Footer
          onClickButtonCancel={() => setSaveTemplateOpen(false)}
          onClickButtonApply={() => void saveAsTemplate()}
          textButtonApply="Сохранить"
          textButtonCancel="Отмена"
          propsButtonApply={{
            loading: templateBusy,
            disabled: !saveTemplateName.trim(),
          }}
        />
      </Dialog>

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

function daysFromNow(days: number): string {
  const date = new Date(Date.now() + days * 24 * 3600 * 1000);
  return date.toISOString().slice(0, 16);
}

function formatRiskDay(day: string): string {
  return new Date(`${day}T00:00:00Z`).toLocaleDateString('ru-RU');
}

function RiskDayTable({
  items,
  onTrustIP,
  onTrustCountry,
}: {
  items: RiskIP[];
  onTrustIP: (ip: string) => void;
  onTrustCountry: (code: string) => void;
}) {
  const columns = useMemo<ColumnDef<RiskIP>[]>(
    () => [
      { id: 'ip', header: 'IP', cell: ({ row }) => row.original.ip },
      {
        id: 'country',
        header: 'Страна',
        cell: ({ row }) => {
          if (row.original.internal) return 'локальный';
          const name = row.original.countryName ?? row.original.country ?? '—';
          return row.original.country && row.original.countryName
            ? `${name} ${row.original.country}`
            : name;
        },
      },
      {
        id: 'asn',
        header: 'Оператор',
        cell: ({ row }) =>
          row.original.asnOrg ?? (row.original.asn ? `AS${row.original.asn}` : '—'),
      },
      {
        id: 'dayFirst',
        header: 'Первый раз за день',
        cell: ({ row }) => (row.original.dayFirst ? formatTime(row.original.dayFirst) : '—'),
      },
      {
        id: 'dayLast',
        header: 'Последний раз за день',
        cell: ({ row }) => (row.original.dayLast ? formatTime(row.original.dayLast) : '—'),
      },
      { id: 'count', header: 'Наблюдений', cell: ({ row }) => row.original.count },
      {
        id: 'new',
        header: 'Новый',
        cell: ({ row }) =>
          row.original.new ? (
            <Label theme="warning" size="s">
              новый
            </Label>
          ) : (
            '—'
          ),
      },
      {
        id: 'trusted',
        header: 'Доверенный',
        cell: ({ row }) =>
          row.original.trusted ? (
            <Label theme="success" size="s">
              да
            </Label>
          ) : (
            '—'
          ),
      },
      {
        id: 'actions',
        header: '',
        cell: ({ row }) => (
          <div className="row">
            {!row.original.trusted && (
              <Button view="outlined" size="s" onClick={() => onTrustIP(row.original.ip)}>
                Доверять IP
              </Button>
            )}
            {row.original.country && !row.original.trusted && (
              <Button
                view="flat"
                size="s"
                onClick={() => onTrustCountry(row.original.country ?? '')}
              >
                Доверять {row.original.country}
              </Button>
            )}
          </div>
        ),
      },
    ],
    [onTrustIP, onTrustCountry],
  );
  const table = useTable({ columns, data: items });
  return (
    <div className="table-scroll">
      <Table table={table} />
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

function compareIPHistory(a: IPHistoryItem, b: IPHistoryItem, id: string): number {
  switch (id) {
    case 'country':
      return (a.countryName ?? '').localeCompare(b.countryName ?? '');
    case 'asn':
      return (a.asnOrg ?? '').localeCompare(b.asnOrg ?? '');
    case 'first':
      return a.firstSeen.localeCompare(b.firstSeen);
    case 'last':
      return a.lastSeen.localeCompare(b.lastSeen);
    case 'count':
      return a.count - b.count;
    default:
      return a.ip.localeCompare(b.ip);
  }
}

function IPHistoryTable({ items }: { items: IPHistoryItem[] }) {
  const isMobile = useMedia('(max-width: 760px)');
  const { sorting, sortHeader, sortRows } = useTableSort([{ id: 'last', desc: true }]);
  const columns = useMemo<ColumnDef<IPHistoryItem>[]>(() => [
    { id: 'ip', header: () => sortHeader('ip', 'IP'), cell: ({ row }) => row.original.ip },
    {
      id: 'country',
      header: () => sortHeader('country', 'Страна'),
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
      header: () => sortHeader('asn', 'Оператор'),
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
            header: () => sortHeader('last', 'Последний раз'),
            cell: ({ row }: { row: { original: IPHistoryItem } }) =>
              new Date(row.original.lastSeen).toLocaleString('ru-RU'),
          },
        ]
      : [
          { id: 'first', header: 'Впервые', cell: ({ row }: { row: { original: IPHistoryItem } }) => new Date(row.original.firstSeen).toLocaleString('ru-RU') },
          { id: 'last', header: 'Последний раз', cell: ({ row }: { row: { original: IPHistoryItem } }) => new Date(row.original.lastSeen).toLocaleString('ru-RU') },
          { id: 'count', header: 'Наблюдений', cell: ({ row }: { row: { original: IPHistoryItem } }) => row.original.count },
        ]),
  ], [isMobile, sorting]);
  const data = useMemo(() => sortRows(items, compareIPHistory), [items, sorting]);
  const table = useTable({ columns, data });
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
