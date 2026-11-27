import { useCallback, useEffect, useState } from 'react';
import { Alert, Button, Card, Checkbox, Icon, Label, Loader, Select, Text, TextArea, TextInput } from '@gravity-ui/uikit';
import { FileArrowDown } from '@gravity-ui/icons';
import {
  api,
  ApiError,
  type BackupStatus,
  type LimitTemplate,
  type SettingsHistoryItem,
  type SettingsSystem,
  type TemplatesResponse,
} from '../api';
import { formatBytes, formatTime } from '../format';
import { templatesYAML } from './settingsYaml';
import { downloadText } from '../utils/download';

type TemplateForm = {
  name: string;
  maxIps: string;
  maxTcpConns: string;
  quotaGiB: string;
  quotaPeriod: string;
  rateUp: string;
  rateDown: string;
  expirationDays: string;
  riskWarn: string;
  riskHigh: string;
  riskDistinct: string;
  riskNewCountries: boolean;
  sourceDeny: string;
};

const emptyForm: TemplateForm = {
  name: '',
  maxIps: '',
  maxTcpConns: '',
  quotaGiB: '',
  quotaPeriod: '',
  rateUp: '',
  rateDown: '',
  expirationDays: '',
  riskWarn: '',
  riskHigh: '',
  riskDistinct: '',
  riskNewCountries: true,
  sourceDeny: '',
};

function toForm(t: LimitTemplate): TemplateForm {
  return {
    name: t.name,
    maxIps: t.maxIps?.toString() ?? '',
    maxTcpConns: t.maxTcpConns?.toString() ?? '',
    quotaGiB: t.dataQuotaBytes ? (t.dataQuotaBytes / 1024 ** 3).toString() : '',
    quotaPeriod: t.quotaPeriod ?? '',
    rateUp: t.rateLimitUpBps ? (t.rateLimitUpBps / 1000).toString() : '',
    rateDown: t.rateLimitDownBps ? (t.rateLimitDownBps / 1000).toString() : '',
    expirationDays: t.expirationDays?.toString() ?? '',
    riskWarn: t.riskWarnIps?.toString() ?? '',
    riskHigh: t.riskHighIps?.toString() ?? '',
    riskDistinct: t.riskDistinct7d?.toString() ?? '',
    riskNewCountries: t.riskNewCountries ?? true,
    sourceDeny: (t.sourceDeny ?? []).join('\n'),
  };
}

function fromForm(f: TemplateForm): LimitTemplate {
  const num = (v: string) => (v.trim() === '' ? undefined : Number(v));
  return {
    name: f.name.trim(),
    maxIps: num(f.maxIps),
    maxTcpConns: num(f.maxTcpConns),
    dataQuotaBytes: f.quotaGiB.trim() === '' ? undefined : Math.round(Number(f.quotaGiB) * 1024 ** 3),
    quotaPeriod: f.quotaPeriod || undefined,
    rateLimitUpBps: f.rateUp.trim() === '' ? undefined : Math.round(Number(f.rateUp) * 1000),
    rateLimitDownBps: f.rateDown.trim() === '' ? undefined : Math.round(Number(f.rateDown) * 1000),
    expirationDays: num(f.expirationDays),
    riskWarnIps: num(f.riskWarn),
    riskHighIps: num(f.riskHigh),
    riskDistinct7d: num(f.riskDistinct),
    riskNewCountries: f.riskNewCountries,
    sourceDeny: f.sourceDeny.split(/[\n,]+/).map((v) => v.trim()).filter(Boolean),
  };
}

export function TemplatesPanel() {
  const [data, setData] = useState<TemplatesResponse | null>(null);
  const [form, setForm] = useState<TemplateForm>(emptyForm);
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [saved, setSaved] = useState(false);

  const load = useCallback(async () => {
    try {
      setData(await api.templates());
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось загрузить шаблоны');
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const update = (key: keyof TemplateForm, value: string | boolean) => {
    setForm((prev) => ({ ...prev, [key]: value }));
    setSaved(false);
  };

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError('');
    try {
      const body = fromForm(form);
      if (editing) {
        await api.updateTemplate(form.name, body);
      } else {
        await api.saveTemplate(body);
      }
      setForm(emptyForm);
      setEditing(false);
      setSaved(true);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось сохранить шаблон');
    } finally {
      setBusy(false);
    }
  };

  const remove = async (name: string) => {
    setBusy(true);
    setError('');
    try {
      await api.deleteTemplate(name);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось удалить шаблон');
    } finally {
      setBusy(false);
    }
  };

  const makeDefault = async (name: string) => {
    setBusy(true);
    setError('');
    try {
      await api.setDefaultTemplate(name);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось назначить шаблон');
    } finally {
      setBusy(false);
    }
  };

  if (!data) return <Loader size="m" />;

  return (
    <Card view="filled" style={{ padding: 16 }}>
      <div className="stack">
        <div className="card-head">
          <Text variant="header-1">Шаблоны лимитов</Text>
          <div style={{ flex: 1 }} />
          <Button
            view="outlined"
            size="s"
            aria-label="Скачать шаблоны YAML"
            onClick={() => downloadText(templatesYAML(data), 'tgadmin-limit-templates.yaml', 'application/yaml;charset=utf-8')}
          >
            <Icon data={FileArrowDown} size={16} />
          </Button>
          <Button
            view="action"
            onClick={() => {
              setForm(emptyForm);
              setEditing(false);
            }}
          >
            Новый шаблон
          </Button>
        </div>
        {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
        {saved && <Text color="positive">сохранено</Text>}

        <div className="stack">
          {data.templates.length === 0 && <Text className="muted">Шаблонов пока нет.</Text>}
          {data.templates.map((t) => (
            <div className="template-row" key={t.name}>
              <div className="stack" style={{ gap: 2 }}>
                <div className="row">
                  <Text variant="subheader-1">{t.name}</Text>
                  {data.default === t.name && (
                    <Label theme="success" size="s">
                      по умолчанию
                    </Label>
                  )}
                </div>
                <Text className="muted">
                  IP {t.maxIps ?? '—'} · соединений {t.maxTcpConns ?? '—'} · квота{' '}
                  {t.dataQuotaBytes ? formatBytes(t.dataQuotaBytes) : '—'} · пороги риска{' '}
                  {t.riskWarnIps ?? '—'}/{t.riskHighIps ?? '—'}/{t.riskDistinct7d ?? '—'}
                </Text>
              </div>
              <div className="row">
                {data.default !== t.name && (
                  <Button view="flat" disabled={busy} onClick={() => void makeDefault(t.name)}>
                    По умолчанию
                  </Button>
                )}
                <Button
                  view="outlined"
                  onClick={() => {
                    setForm(toForm(t));
                    setEditing(true);
                  }}
                >
                  Изменить
                </Button>
                <Button view="flat-danger" disabled={busy} onClick={() => void remove(t.name)}>
                  Удалить
                </Button>
              </div>
            </div>
          ))}
        </div>

        <form className="stack" onSubmit={submit}>
          <Text variant="subheader-1">{editing ? `Изменить ${form.name}` : 'Новый шаблон'}</Text>
          <div className="form-grid">
            <label className="field">
              <span>Имя</span>
              <TextInput value={form.name} onUpdate={(v) => update('name', v)} disabled={editing} />
            </label>
            <label className="field">
              <span>Максимум IP</span>
              <TextInput value={form.maxIps} onUpdate={(v) => update('maxIps', v)} />
            </label>
            <label className="field">
              <span>Максимум соединений</span>
              <TextInput value={form.maxTcpConns} onUpdate={(v) => update('maxTcpConns', v)} />
            </label>
            <label className="field">
              <span>Квота, ГиБ</span>
              <TextInput value={form.quotaGiB} onUpdate={(v) => update('quotaGiB', v)} />
            </label>
            <label className="field">
              <span>Период квоты</span>
              <Select
                aria-label="Период квоты"
                value={form.quotaPeriod ? [form.quotaPeriod] : []}
                options={[
                  { value: '', content: 'без сброса' },
                  { value: 'daily', content: 'сутки' },
                  { value: 'weekly', content: 'неделя' },
                  { value: 'monthly', content: 'месяц' },
                ]}
                onUpdate={([value]) => update('quotaPeriod', value ?? '')}
                placeholder="без сброса"
                width="max"
              />
            </label>
            <label className="field">
              <span>Срок, дней</span>
              <TextInput
                value={form.expirationDays}
                onUpdate={(v) => update('expirationDays', v)}
              />
            </label>
            <label className="field">
              <span>Скорость вверх, Кбит/с</span>
              <TextInput value={form.rateUp} onUpdate={(v) => update('rateUp', v)} />
            </label>
            <label className="field">
              <span>Скорость вниз, Кбит/с</span>
              <TextInput value={form.rateDown} onUpdate={(v) => update('rateDown', v)} />
            </label>
            <label className="field">
              <span>Риск: предупреждение</span>
              <TextInput value={form.riskWarn} onUpdate={(v) => update('riskWarn', v)} />
            </label>
            <label className="field">
              <span>Риск: высокий</span>
              <TextInput value={form.riskHigh} onUpdate={(v) => update('riskHigh', v)} />
            </label>
            <label className="field">
              <span>Риск: адресов за 7 дней</span>
              <TextInput value={form.riskDistinct} onUpdate={(v) => update('riskDistinct', v)} />
            </label>
            <Checkbox
              controlProps={{ 'aria-label': 'новая страна считается риском' }}
              checked={form.riskNewCountries}
              onUpdate={(checked) => update('riskNewCountries', checked)}
              content="новая страна считается риском"
            />
          </div>
          <label className="field">
            <span>Запрещённые сети, по одной на строку</span>
            <TextArea
              rows={2}
              value={form.sourceDeny}
              onUpdate={(value) => update('sourceDeny', value)}
              placeholder="203.0.113.0/24"
            />
          </label>
          <div className="form-actions">
            <Button view="action" type="submit" loading={busy} disabled={!form.name.trim()}>
              {editing ? 'Сохранить шаблон' : 'Создать шаблон'}
            </Button>
            {editing && (
              <Button
                view="flat"
                onClick={() => {
                  setForm(emptyForm);
                  setEditing(false);
                }}
              >
                Отмена
              </Button>
            )}
          </div>
        </form>
      </div>
    </Card>
  );
}

export function BackupsPanel() {
  const [data, setData] = useState<BackupStatus | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      setData(await api.backups());
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось загрузить статус');
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const snapshot = async () => {
    setBusy(true);
    setError('');
    try {
      await api.backupSnapshot();
      window.setTimeout(() => void load(), 1500);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось запустить снимок');
    } finally {
      setBusy(false);
    }
  };

  if (!data) return null;

  return (
    <Card view="filled" style={{ padding: 16 }}>
      <div className="stack">
        <div className="card-head">
          <Text variant="header-1">Бэкапы</Text>
          <div style={{ flex: 1 }} />
          <Button view="outlined" loading={busy} onClick={() => void snapshot()}>
            Снимок сейчас
          </Button>
        </div>
        {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
        <div className="stat-grid">
          <div className="stat-tile">
            <span className="stat-label">Последний снимок Valkey</span>
            <span className="stat-value">
              {data.valkey.lastSave ? formatTime(data.valkey.lastSave) : '—'}
            </span>
          </div>
          <div className="stat-tile">
            <span className="stat-label">Статус</span>
            <span className="stat-value">
              {data.valkey.saveInProgress ? 'выполняется' : data.valkey.lastStatus || '—'}
            </span>
          </div>
          <div className="stat-tile">
            <span className="stat-label">Ключей</span>
            <span className="stat-value">{data.valkey.keys}</span>
          </div>
          <div className="stat-tile">
            <span className="stat-label">Память</span>
            <span className="stat-value">{data.valkey.memory ?? '—'}</span>
          </div>
        </div>
        <Text className="muted">
          Полный снимок с проверкой, сжатием и ротацией делает хост по расписанию. Кнопка выше
          запускает BGSAVE внутри Valkey.
        </Text>
        {data.host.available ? (
          <Text className="muted">
            Хост: {data.host.lastRun ? formatTime(data.host.lastRun) : '—'} ·{' '}
            {data.host.result || '—'} ·{' '}
            {data.host.size ? formatBytes(data.host.size) : '—'} · хранить{' '}
            {data.host.keepDays ?? '—'} дней · проверка восстановления{' '}
            {data.host.restoreTest ? 'да' : 'нет'}
          </Text>
        ) : (
          <Text className="muted">
            Статус хоста недоступен: смонтируйте файл или смотрите журнал valkey-backup.service.
          </Text>
        )}
      </div>
    </Card>
  );
}

export function SystemPanel() {
  const [data, setData] = useState<SettingsSystem | null>(null);
  useEffect(() => {
    api
      .system()
      .then(setData)
      .catch(() => setData(null));
  }, []);
  if (!data) return null;
  return (
    <Card view="filled" style={{ padding: 16 }}>
      <div className="stack">
        <Text variant="header-1">Система</Text>
        <div className="stat-grid">
          <div className="stat-tile">
            <span className="stat-label">Версия</span>
            <span className="stat-value">{data.version}</span>
          </div>
          <div className="stat-tile">
            <span className="stat-label">Uptime</span>
            <span className="stat-value">{data.uptime}</span>
          </div>
          <div className="stat-tile">
            <span className="stat-label">Сборщик</span>
            <span className="stat-value">{data.collector?.up ? 'работает' : 'остановлен'}</span>
          </div>
          <div className="stat-tile">
            <span className="stat-label">GeoIP</span>
            <span className="stat-value">{data.geoip?.loaded ? 'включён' : 'выключен'}</span>
          </div>
          <div className="stat-tile">
            <span className="stat-label">Ключей Valkey</span>
            <span className="stat-value">{data.valkey.keys}</span>
          </div>
          <div className="stat-tile">
            <span className="stat-label">Память Valkey</span>
            <span className="stat-value">{data.valkey.memory ?? '—'}</span>
          </div>
          <div className="stat-tile">
            <span className="stat-label">Пользователей UI</span>
            <span className="stat-value">{data.counts.uiUsers}</span>
          </div>
          <div className="stat-tile">
            <span className="stat-label">Событий аудита</span>
            <span className="stat-value">{data.counts.auditEvents}</span>
          </div>
          <div className="stat-tile">
            <span className="stat-label">Шаблонов</span>
            <span className="stat-value">{data.counts.templates}</span>
          </div>
        </div>
      </div>
    </Card>
  );
}

export function HistoryPanel() {
  const [items, setItems] = useState<SettingsHistoryItem[] | null>(null);
  useEffect(() => {
    api
      .settingsHistory()
      .then(setItems)
      .catch(() => setItems([]));
  }, []);
  if (!items) return null;
  return (
    <Card view="filled" style={{ padding: 16 }}>
      <div className="stack">
        <Text variant="header-1">История изменений</Text>
        {items.length === 0 ? (
          <Text className="muted">Изменений пока нет.</Text>
        ) : (
          items.map((item) => (
            <div className="timeline-row" key={`${item.At}-${item.Target}-${item.Result}`}>
              <span className="timeline-date">{formatTime(item.At)}</span>
              <div className="timeline-events">
                <Text>{item.Target}</Text>
                <span className="muted">{item.Result}</span>
                <span className="muted">
                  {item.Actor} · {item.IP}
                </span>
              </div>
            </div>
          ))
        )}
      </div>
    </Card>
  );
}
