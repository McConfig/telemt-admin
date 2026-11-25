import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Alert, Button, Card, Text, TextInput } from '@gravity-ui/uikit';
import { api, ApiError, type LimitTemplate, type Me } from '../api';

function daysFromNow(days: number): string {
  const date = new Date(Date.now() + days * 24 * 3600 * 1000);
  return date.toISOString().slice(0, 16);
}

export default function NewUserPage({ me }: { me: Me }) {
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [secret, setSecret] = useState('');
  const [maxIps, setMaxIps] = useState('');
  const [maxConns, setMaxConns] = useState('');
  const [quotaGB, setQuotaGB] = useState('');
  const [period, setPeriod] = useState('');
  const [rateUp, setRateUp] = useState('');
  const [rateDown, setRateDown] = useState('');
  const [expiration, setExpiration] = useState('');
  const [sourceDeny, setSourceDeny] = useState('');
  const [templates, setTemplates] = useState<LimitTemplate[]>([]);
  const [templateName, setTemplateName] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api
      .templates()
      .then((data) => {
        setTemplates(data.templates);
        const initial = data.templates.find((t) => t.name === data.default) ?? data.templates[0];
        if (initial) {
          setTemplateName(initial.name);
          applyTemplate(initial);
        }
      })
      .catch(() => setTemplates([]));
  }, []);

  function applyTemplate(t: LimitTemplate) {
    setMaxIps(t.maxIps?.toString() ?? '');
    setMaxConns(t.maxTcpConns?.toString() ?? '');
    setQuotaGB(t.dataQuotaBytes ? (t.dataQuotaBytes / 1024 ** 3).toFixed(2) : '');
    setPeriod(t.quotaPeriod ?? '');
    setRateUp(t.rateLimitUpBps ? (t.rateLimitUpBps / 1000).toString() : '');
    setRateDown(t.rateLimitDownBps ? (t.rateLimitDownBps / 1000).toString() : '');
    setExpiration(t.expirationDays ? daysFromNow(t.expirationDays) : '');
    setSourceDeny((t.sourceDeny ?? []).join('\n'));
  }

  if (me.user.role !== 'admin') {
    return <Alert theme="warning" title="Нет прав" message="Создавать пользователей может только admin" />;
  }

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError('');
    try {
      const user = await api.create({
        name: name.trim(),
        secret: secret.trim() || undefined,
        maxUniqueIps: maxIps ? Number(maxIps) : undefined,
        maxTcpConns: maxConns ? Number(maxConns) : undefined,
        dataQuotaBytes: quotaGB ? Math.round(Number(quotaGB) * 1024 ** 3) : undefined,
        quotaPeriod: period || undefined,
        rateLimitUpBps: rateUp ? Math.round(Number(rateUp) * 1000) : undefined,
        rateLimitDownBps: rateDown ? Math.round(Number(rateDown) * 1000) : undefined,
        expiration: expiration ? new Date(expiration).toISOString() : undefined,
        sourceDeny: sourceDeny.split(/[\n,]+/).map((v) => v.trim()).filter(Boolean),
      });
      navigate(`/users/${encodeURIComponent(user.username)}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось создать');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="stack" style={{ maxWidth: 640 }}>
      <Text variant="header-2">Новый пользователь</Text>
      <Card view="filled" style={{ padding: 24 }}>
        <form className="stack" onSubmit={submit}>
          {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
          {templates.length > 0 && (
            <label className="field">
              <span>Шаблон лимитов</span>
              <select
                value={templateName}
                onChange={(event) => {
                  const next = templates.find((t) => t.name === event.target.value);
                  setTemplateName(event.target.value);
                  if (next) applyTemplate(next);
                }}
              >
                {templates.map((t) => (
                  <option key={t.name} value={t.name}>
                    {t.name}
                  </option>
                ))}
              </select>
            </label>
          )}
          <TextInput placeholder="Имя" value={name} onUpdate={setName} />
          <TextInput placeholder="Секрет (32 hex, пусто — сгенерировать)" value={secret} onUpdate={setSecret} />
          <TextInput placeholder="Максимум уникальных IP" value={maxIps} onUpdate={setMaxIps} />
          <TextInput placeholder="Максимум соединений" value={maxConns} onUpdate={setMaxConns} />
          <TextInput placeholder="Квота, ГиБ" value={quotaGB} onUpdate={setQuotaGB} />
          <label className="field">
            <span>Период квоты</span>
            <select value={period} onChange={(e) => setPeriod(e.target.value)}>
              <option value="">без сброса</option>
              <option value="daily">сутки</option>
              <option value="weekly">неделя</option>
              <option value="monthly">месяц</option>
            </select>
          </label>
          <TextInput placeholder="Скорость вверх, Кбит/с" value={rateUp} onUpdate={setRateUp} />
          <TextInput placeholder="Скорость вниз, Кбит/с" value={rateDown} onUpdate={setRateDown} />
          <label className="field">
            <span>Действует до</span>
            <input
              type="datetime-local"
              value={expiration}
              onChange={(e) => setExpiration(e.target.value)}
            />
          </label>
          <label className="field">
            <span>Запрещённые сети, по одной на строку</span>
            <textarea
              rows={2}
              value={sourceDeny}
              onChange={(e) => setSourceDeny(e.target.value)}
              placeholder="203.0.113.0/24"
            />
          </label>
          <div className="row">
            <Button view="action" type="submit" loading={busy} disabled={!name.trim()}>
              Создать
            </Button>
            <Button view="flat" onClick={() => navigate('/')}>
              Отмена
            </Button>
          </div>
        </form>
      </Card>
    </div>
  );
}
