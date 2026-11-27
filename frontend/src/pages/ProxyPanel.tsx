import { useCallback, useEffect, useMemo, useState, type CSSProperties } from 'react';
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Dialog,
  Icon,
  Label,
  Loader,
  Popover,
  Select,
  Text,
  TextArea,
  TextInput,
} from '@gravity-ui/uikit';
import { ChevronDown, ChevronRight, FileArrowDown } from '@gravity-ui/icons';
import { api, ApiError, type ProxyConfig, type ProxySchema, type ProxySchemaField } from '../api';
import { proxyDescription, useLocale } from '../i18n';
import { configToTOML, toTOML } from './proxyToml';
import { downloadText } from '../utils/download';

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

function prettyJSON(value: unknown, fallback: unknown): string {
  return JSON.stringify(value ?? fallback, null, 2);
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

type Kind = 'string' | 'bool' | 'number' | 'enum' | 'json';

function isContainer(field: ProxySchemaField): boolean {
  const type = String(field.value.type ?? '');
  return type === 'array' || type === 'object';
}

function hasLeafChildren(field: ProxySchemaField, fields: ProxySchemaField[]): boolean {
  const prefix = field.path + '.';
  return fields.some(
    (other) =>
      other.path.startsWith(prefix) &&
      !other.path.includes('[]') &&
      !other.path.includes('{') &&
      !isContainer(other),
  );
}

function kindOf(field: ProxySchemaField): Kind {
  const type = String(field.value.type ?? 'string');
  if (type === 'boolean') return 'bool';
  if (type === 'integer' || type === 'number') return 'number';
  if (type === 'array' || type === 'object') return 'json';
  if (Array.isArray(field.value.enum)) return 'enum';
  return 'string';
}

function defaultFor(field: ProxySchemaField): unknown {
  return String(field.value.type ?? '') === 'array' ? [] : {};
}

type JsonEditor = {
  path: string;
  text: string;
  error: string;
  expected: 'array' | 'object';
};


export default function ProxyPanel() {
  const [schema, setSchema] = useState<ProxySchema | null>(null);
  const [config, setConfig] = useState<ProxyConfig | null>(null);
  const [draft, setDraft] = useState<Record<string, unknown>>({});
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [reload, setReload] = useState('');
  const [search, setSearch] = useState('');
  const [jsonEditor, setJsonEditor] = useState<JsonEditor | null>(null);
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const { locale } = useLocale();

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

  const writable = useMemo(
    () =>
      (schema?.fields ?? []).filter(
        (field) => field.api.config_write && !field.path.includes('[]') && !field.path.includes('{'),
      ),
    [schema],
  );

  const rows = useMemo(
    () => writable.filter((field) => !isContainer(field) || !hasLeafChildren(field, writable)),
    [writable],
  );

  const readOnly = useMemo(
    () =>
      (schema?.fields ?? []).filter(
        (field) =>
          !field.api.config_write &&
          !field.path.includes('[]') &&
          !field.path.includes('{') &&
          !(isContainer(field) && hasLeafChildren(field, schema?.fields ?? [])),
      ),
    [schema],
  );

  const nameWidth = useMemo(() => {
    const longest = Math.max(24, ...[...rows, ...readOnly].map((field) => field.path.length));
    return `${longest * 8 + 56}px`;
  }, [rows, readOnly]);

  const groups = useMemo(() => {
    const out: Record<string, ProxySchemaField[]> = {};
    for (const field of rows) {
      const section = field.path.split('.')[0];
      (out[section] ??= []).push(field);
    }
    for (const section of Object.keys(out)) {
      out[section].sort((a, b) => a.path.localeCompare(b.path));
    }
    return out;
  }, [rows]);

  const current = (field: ProxySchemaField) => {
    if (draft[field.path] !== undefined) return draft[field.path];
    return getPath(config?.config ?? {}, field.path) ?? field.value.default;
  };

  const update = (field: ProxySchemaField, raw: unknown) => {
    const kind = kindOf(field);
    let value: unknown = raw;
    if (kind === 'number') value = raw === '' ? undefined : Number(raw);
    setDraft((prev) => ({ ...prev, [field.path]: value }));
  };

  const patch = useMemo(() => {
    const out: Record<string, unknown> = {};
    for (const field of rows) {
      const next = draft[field.path];
      if (next === undefined) continue;
      const old = getPath(config?.config ?? {}, field.path);
      if (JSON.stringify(next) !== JSON.stringify(old)) setPath(out, field.path, next);
    }
    return out;
  }, [draft, rows, config]);

  const changedPaths = useMemo(() => {
    const out: string[] = [];
    const walk = (prefix: string, value: Record<string, unknown>) => {
      for (const [key, child] of Object.entries(value)) {
        const path = prefix ? `${prefix}.${key}` : key;
        if (typeof child === 'object' && child !== null && !Array.isArray(child)) {
          walk(path, child as Record<string, unknown>);
        } else {
          out.push(path);
        }
      }
    };
    walk('', patch);
    return out;
  }, [patch]);

  const rowChanged = (field: ProxySchemaField) => {
    if (draft[field.path] === undefined) return false;
    const original = getPath(config?.config ?? {}, field.path);
    return JSON.stringify(draft[field.path]) !== JSON.stringify(original);
  };

  const matches = (field: ProxySchemaField) => {
    const needle = search.trim().toLowerCase();
    if (!needle) return true;
    const value = valueText(current(field)).toLowerCase();
    return (
      field.path.toLowerCase().includes(needle) ||
      String(field.description ?? '').toLowerCase().includes(needle) ||
      value.includes(needle)
    );
  };

  const openJSON = (field: ProxySchemaField) => {
    const expected = String(field.value.type) === 'array' ? 'array' : 'object';
    setJsonEditor({
      path: field.path,
      text: prettyJSON(current(field), defaultFor(field)),
      error: '',
      expected,
    });
  };

  const changeJSON = (text: string) => {
    if (!jsonEditor) return;
    let nextError = '';
    try {
      const parsed = JSON.parse(text);
      if (jsonEditor.expected === 'array' && !Array.isArray(parsed)) {
        nextError = 'ожидается JSON-массив';
      } else if (
        jsonEditor.expected === 'object' &&
        (parsed === null || Array.isArray(parsed) || typeof parsed !== 'object')
      ) {
        nextError = 'ожидается JSON-объект';
      }
    } catch (err) {
      nextError = err instanceof Error ? err.message : 'некорректный JSON';
    }
    setJsonEditor({ ...jsonEditor, text, error: nextError });
  };

  const applyJSON = () => {
    if (!jsonEditor || jsonEditor.error) return;
    setDraft((prev) => ({ ...prev, [jsonEditor.path]: JSON.parse(jsonEditor.text) }));
    setJsonEditor(null);
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

  const fullConfigValues = () => {
    const root: Record<string, unknown> = JSON.parse(JSON.stringify(config?.config ?? {}));
    for (const field of rows) {
      if (draft[field.path] !== undefined) setPath(root, field.path, draft[field.path]);
    }
    return root;
  };

  const sectionValues = (section: string) => fullConfigValues()[section];

  const renderRow = (field: ProxySchemaField, readonly = false) => {
    const kind = kindOf(field);
    const value = current(field);
    const help = proxyDescription(field.path, field.description, locale) || field.non_editable_reason;
    return (
      <div className={`proxy-row${rowChanged(field) ? ' proxy-row_changed' : ''}`} key={field.path}>
        <div className="proxy-cell proxy-name">
          <div className="proxy-name-line">
            <Text>{field.path}</Text>
            {help && (
              <Popover
                trigger="click"
                placement="right"
                content={<Text className="proxy-help-text">{help}</Text>}
              >
                <Button view="flat" size="s" className="proxy-help" aria-label={`Описание ${field.path}`}>
                  ?
                </Button>
              </Popover>
            )}
          </div>
        </div>
        <div className="proxy-cell proxy-value">
          {kind === 'bool' ? (
            readonly ? (
              <Text>{Boolean(value) ? 'true' : 'false'}</Text>
            ) : (
              <Select
                aria-label={field.path}
                value={[String(Boolean(value))]}
                options={[
                  { value: 'true', content: 'true' },
                  { value: 'false', content: 'false' },
                ]}
                onUpdate={([next]) => update(field, next === 'true')}
                width="max"
              />
            )
          ) : kind === 'enum' ? (
            readonly ? (
              <Text>{String(value ?? '')}</Text>
            ) : (
              <Select
                aria-label={field.path}
                value={[String(value ?? '')]}
                options={(field.value.enum as unknown[]).map((item) => ({
                  value: String(item),
                  content: String(item),
                }))}
                onUpdate={([next]) => update(field, next ?? '')}
                width="max"
              />
            )
          ) : kind === 'json' ? (
            <div className="row proxy-json">
              <pre className="proxy-json-preview">{value === undefined ? 'не задано' : JSON.stringify(value, null, 2)}</pre>
              {!readonly && (
                <Button view="outlined" size="s" onClick={() => openJSON(field)}>
                  Изменить JSON
                </Button>
              )}
            </div>
          ) : readonly ? (
            <Text>{valueText(value)}</Text>
          ) : (
            <TextInput
              value={valueText(value)}
              type={kind === 'number' ? 'number' : 'text'}
              onUpdate={(next) => update(field, next)}
              controlProps={{ 'aria-label': field.path }}
            />
          )}
        </div>
      </div>
    );
  };

  const header = (
    <div className="proxy-row proxy-row_head" key="header">
      <div className="proxy-cell">Параметр</div>
      <div className="proxy-cell">Значение</div>
    </div>
  );

  return (
    <Card view="filled" style={{ padding: 16 }}>
      <div className="stack">
        <div className="row">
          <Text variant="header-1">Прокси telemt</Text>
          <div style={{ flex: 1 }} />
          <Button
            view="outlined"
            size="s"
            aria-label="Скачать полный TOML"
            onClick={() => downloadText(configToTOML(fullConfigValues()), 'telemt-config.toml', 'application/toml;charset=utf-8')}
          >
            <Icon data={FileArrowDown} size={16} />
            TOML
          </Button>
          <Label theme="info">ревизия {config.revision.slice(0, 12)}</Label>
        </div>
        {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
        {message && <Alert theme="success" message={message} onClose={() => setMessage('')} />}
        {reload && <Text className="muted">{reload}</Text>}
        <TextInput
          placeholder="Поиск по параметру или значению"
          value={search}
          onUpdate={setSearch}
          controlProps={{ 'aria-label': 'Поиск по параметру или значению' }}
        />

        <div className="proxy-table" style={{ '--proxy-name-width': nameWidth } as CSSProperties}>
          {header}
          {Object.entries(groups).map(([section, items]) => {
            const visible = items.filter(matches);
            if (visible.length === 0) return null;
            const isCollapsed = Boolean(collapsed[section]);
            return (
              <div className="proxy-group" key={section}>
                <div className="proxy-group-head">
                  <Button
                    view="flat"
                    className="proxy-group-toggle"
                    aria-expanded={!isCollapsed}
                    onClick={() => setCollapsed((prev) => ({ ...prev, [section]: !prev[section] }))}
                  >
                    <span className="proxy-group-title">
                      <Icon data={isCollapsed ? ChevronRight : ChevronDown} size={16} />
                      <Text variant="header-2">{section}</Text>
                    </span>
                  </Button>
                  <Button
                    view="flat"
                    size="s"
                    aria-label={`Скачать TOML ${section}`}
                    onClick={() => downloadText(toTOML(section, sectionValues(section)), `telemt-${section}.toml`)}
                  >
                    <Icon data={FileArrowDown} size={16} />
                  </Button>
                </div>
                {!isCollapsed && visible.map((field) => renderRow(field))}
              </div>
            );
          })}
        </div>

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
            disabled={changedPaths.length === 0 || (changedPaths.some(isDangerous) && !confirm)}
            onClick={() => void apply()}
          >
            Применить
          </Button>
        </div>

        {readOnly.length > 0 && (
          <details>
            <summary>Только через файл или перезапуск</summary>
            <div
              className="proxy-table"
              style={{ marginTop: 8, '--proxy-name-width': nameWidth } as CSSProperties}
            >
              {readOnly.filter(matches).map((field) => renderRow(field, true))}
            </div>
          </details>
        )}
      </div>

      <Dialog
        open={jsonEditor !== null}
        onOpenChange={(next) => !next && setJsonEditor(null)}
        fullWidth
        modalClassName="proxy-json-modal"
        className="proxy-json-dialog"
      >
        <Dialog.Header caption={jsonEditor ? `JSON: ${jsonEditor.path}` : 'JSON'} />
        <Dialog.Body>
          <div className="stack proxy-json-dialog-content">
            <div className="proxy-json-error">
              {jsonEditor?.error ? (
                <Alert theme="danger" message={jsonEditor.error} />
              ) : (
                <Text className="muted">JSON проверяется автоматически</Text>
              )}
            </div>
            <TextArea
              className="proxy-json-editor"
              minRows={20}
              maxRows={20}
              value={jsonEditor?.text ?? ''}
              onUpdate={changeJSON}
              controlProps={{ 'aria-label': 'JSON' }}
            />
            <div className="row">
              <Button
                view="outlined"
                onClick={() => {
                  if (!jsonEditor) return;
                  try {
                    changeJSON(JSON.stringify(JSON.parse(jsonEditor.text), null, 2));
                  } catch {
                    // ошибка уже показана при вводе
                  }
                }}
              >
                Форматировать
              </Button>
            </div>
          </div>
        </Dialog.Body>
        <Dialog.Footer
          onClickButtonCancel={() => setJsonEditor(null)}
          onClickButtonApply={applyJSON}
          textButtonApply="Применить"
          textButtonCancel="Отмена"
          propsButtonApply={{ disabled: Boolean(jsonEditor?.error) }}
        />
      </Dialog>
    </Card>
  );
}
