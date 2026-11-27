import { useCallback, useEffect, useMemo, useState } from 'react';
import { Alert, Button, Card, Checkbox, Label, Loader, Select, Text, TextInput } from '@gravity-ui/uikit';
import { api, ApiError, type ProxyConfig, type ProxySchema, type ProxySchemaField } from '../api';

function getPath(source: Record<string, unknown>, path: string): unknown {
  let current: unknown = source;
  for (const key of path.split('.')) {
    if (typeof current !== 'object' || current === null) return undefined;
    current = (current as Record<string, unknown>)[key];
  }
  return current;
}

function setPath(target: Record<string, unknown>, path: string, value: unknown) {
  const keys = path.split('.');
  let current = target;
  for (const key of keys.slice(0, -1)) {
    const next = current[key];
    if (typeof next !== 'object' || next === null) current[key] = {};
    current = current[key] as Record<string, unknown>;
  }
  current[keys[keys.length - 1]] = value;
}

function valueText(value: unknown): string {
  if (value === undefined || value === null) return '';
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}

const dangerous = [
  'censorship.tls_domain',
  'general.modes.',
  'server.listeners',
  'general.links.public_host',
  'general.links.public_port',
];

function isDangerous(path: string) {
  return dangerous.some((prefix) => path === prefix || path.startsWith(prefix));
}

type Kind = 'string' | 'bool' | 'number' | 'enum';

function kindOf(field: ProxySchemaField): Kind {
  const type = String(field.value.type ?? 'string');
  if (type === 'boolean') return 'bool';
  if (type === 'integer' || type === 'number') return 'number';
  if (Array.isArray(field.value.enum)) return 'enum';
  return 'string';
}

function editableSimple(field: ProxySchemaField): boolean {
  return field.api.config_write && !field.path.includes('[]') && !field.path.includes('{');
}

export default function ProxyPanel() {
  const [schema, setSchema] = useState<ProxySchema | null>(null);
  const [config, setConfig] = useState<ProxyConfig | null>(null);
  const [draft, setDraft] = useState<Record<string, unknown>>({});
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [reload, setReload] = useState('');

  const load = useCallback(async () => {
    try {
      const [nextSchema, nextConfig] = await Promise.all([api.proxySchema(), api.proxyConfig()]);
      setSchema(nextSchema);
      setConfig(nextConfig);
      setDraft({});
      setError('');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось загрузить конфигурацию прокси');
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const fields = useMemo(
    () => (schema?.fields ?? []).filter(editableSimple),
    [schema],
  );
  const readOnly = useMemo(
    () => (schema?.fields ?? []).filter((field) => !field.api.config_write && !field.path.includes('{')),
    [schema],
  );
  const groups = useMemo(() => {
    const out: Record<string, ProxySchemaField[]> = {};
    for (const field of fields) {
      const section = field.path.split('.')[0];
      (out[section] ??= []).push(field);
    }
    return out;
  }, [fields]);

  const current = (field: ProxySchemaField) => {
    if (draft[field.path] !== undefined) return draft[field.path];
    return getPath(config?.config ?? {}, field.path) ?? field.value.default;
  };

  const patch = useMemo(() => {
    const out: Record<string, unknown> = {};
    for (const field of fields) {
      const next = draft[field.path];
      if (next === undefined) continue;
      const old = getPath(config?.config ?? {}, field.path);
      if (JSON.stringify(next) !== JSON.stringify(old)) setPath(out, field.path, next);
    }
    return out;
  }, [draft, fields, config]);

  const changedPaths = useMemo(() => {
    const out: string[] = [];
    const walk = (prefix: string, value: Record<string, unknown>) => {
      for (const [key, child] of Object.entries(value)) {
        const path = prefix ? `${prefix}.${key}` : key;
        if (typeof child === 'object' && child !== null) walk(path, child as Record<string, unknown>);
        else out.push(path);
      }
    };
    walk('', patch);
    return out;
  }, [patch]);

  const update = (field: ProxySchemaField, raw: unknown) => {
    const kind = kindOf(field);
    let value: unknown = raw;
    if (kind === 'number') value = raw === '' ? undefined : Number(raw);
    setDraft((prev) => ({ ...prev, [field.path]: value }));
  };

  const validate = async () => {
    setBusy(true);
    setError('');
    setMessage('');
    try {
      await api.proxyValidate(patch);
      setMessage('Проверка пройдена');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'проверка не прошла');
    } finally {
      setBusy(false);
    }
  };

  const apply = async () => {
    setBusy(true);
    setError('');
    setMessage('');
    try {
      const out = await api.proxyApply(patch, confirm);
      setMessage(`Применено, ревизия ${out.revision.slice(0, 12)}`);
      setConfirm(false);
      await load();
      if (out.reloadId) {
        const status = await api.proxyReload(out.reloadId).catch(() => null);
        if (status) setReload(`Перезагрузка ${out.reloadId}: ${status.state}`);
      }
    } catch (err) {
      if (err instanceof ApiError && err.code === 'proxy_conflict') {
        setError('конфиг изменился, обновите страницу');
      } else {
        setError(err instanceof ApiError ? err.message : 'не удалось применить конфигурацию');
      }
    } finally {
      setBusy(false);
    }
  };

  if (!schema || !config) {
    return (
      <Card view="filled" style={{ padding: 16 }}>
        {error ? <Alert theme="danger" message={error} onClose={() => setError('')} /> : <Loader size="m" />}
      </Card>
    );
  }

  return (
    <Card view="filled" style={{ padding: 16 }}>
      <div className="stack">
        <div className="row">
          <Text variant="header-1">Прокси telemt</Text>
          <div style={{ flex: 1 }} />
          <Label theme="info">ревизия {config.revision.slice(0, 12)}</Label>
        </div>
        {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
        {message && <Alert theme="success" message={message} onClose={() => setMessage('')} />}
        {reload && <Text className="muted">{reload}</Text>}

        {Object.entries(groups).map(([section, items]) => (
          <div className="stack" key={section}>
            <Text variant="header-2">{section}</Text>
            {items.map((field) => {
              const kind = kindOf(field);
              const label = field.path;
              const value = current(field);
              return (
                <label className="stack" key={field.path}>
                  <Text className="muted">{label}</Text>
                  {kind === 'bool' ? (
                    <Checkbox
                      checked={Boolean(value)}
                      onUpdate={(checked) => update(field, checked)}
                      content="включено"
                      controlProps={{ 'aria-label': label }}
                    />
                  ) : kind === 'enum' ? (
                    <Select
                      aria-label={label}
                      value={[String(value ?? '')]}
                      options={(field.value.enum as unknown[]).map((item) => ({
                        value: String(item),
                        content: String(item),
                      }))}
                      onUpdate={([next]) => update(field, next ?? '')}
                      width="max"
                    />
                  ) : (
                    <TextInput
                      value={valueText(value)}
                      type={kind === 'number' ? 'number' : 'text'}
                      onUpdate={(next) => update(field, next)}
                      controlProps={{ 'aria-label': label }}
                    />
                  )}
                </label>
              );
            })}
          </div>
        ))}

        {changedPaths.length > 0 && (
          <div className="stack">
            <Text variant="subheader-1">Изменения</Text>
            {changedPaths.map((path) => (
              <Text key={path} className="muted">
                {path}
              </Text>
            ))}
            {changedPaths.some(isDangerous) && (
              <Checkbox
                checked={confirm}
                onUpdate={setConfirm}
                content="Подтверждаю опасные изменения"
                controlProps={{ 'aria-label': 'Подтвердить опасные изменения' }}
              />
            )}
          </div>
        )}

        <div className="row">
          <Button view="outlined" loading={busy} disabled={changedPaths.length === 0} onClick={() => void validate()}>
            Проверить
          </Button>
          <Button
            view="action"
            loading={busy}
            disabled={
              changedPaths.length === 0 ||
              (changedPaths.some(isDangerous) && !confirm)
            }
            onClick={() => void apply()}
          >
            Применить
          </Button>
        </div>

        {readOnly.length > 0 && (
          <details>
            <summary>Только через файл или перезапуск</summary>
            <div className="stack">
              {readOnly.slice(0, 40).map((field) => (
                <Text key={field.path} className="muted">
                  {field.path}
                  {field.non_editable_reason ? `: ${field.non_editable_reason}` : ''}
                </Text>
              ))}
            </div>
          </details>
        )}
      </div>
    </Card>
  );
}
