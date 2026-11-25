import { useCallback, useEffect, useState } from 'react';
import { Alert, Button, Card, Label, Loader, Text, TextInput } from '@gravity-ui/uikit';
import { api, ApiError, type SettingField, type SettingsCatalog } from '../api';
import { BackupsPanel, HistoryPanel, SystemPanel, TemplatesPanel } from './SettingsExtras';

function isDirty(field: SettingField, value: unknown): boolean {
  if (field.status !== 'active') return false;
  if (field.kind === 'bool') return value !== field.value;
  return String(value ?? '') !== String(field.value ?? '');
}

function valueText(field: SettingField, value: unknown): string {
  if (value === undefined || value === null || value === '') return '—';
  if (field.kind === 'bool') return value ? 'да' : 'нет';
  return String(value);
}

export default function SettingsPage() {
  const [catalog, setCatalog] = useState<SettingsCatalog | null>(null);
  const [draft, setDraft] = useState<Record<string, unknown>>({});
  const [reset, setReset] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [saved, setSaved] = useState(false);
  const [conflict, setConflict] = useState(false);

  const applyCatalog = useCallback((data: SettingsCatalog) => {
    setCatalog(data);
    const values: Record<string, unknown> = {};
    for (const section of data.sections) {
      for (const field of section.settings) {
        if (field.status === 'active') values[field.key] = field.value;
      }
    }
    setDraft(values);
    setReset([]);
    setConflict(false);
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      applyCatalog(await api.settings());
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось загрузить настройки');
    } finally {
      setLoading(false);
    }
  }, [applyCatalog]);

  useEffect(() => {
    void load();
  }, [load]);

  const update = (key: string, value: unknown) => {
    setDraft((prev) => ({ ...prev, [key]: value }));
    setReset((prev) => prev.filter((item) => item !== key));
    setSaved(false);
  };

  const resetField = (field: SettingField) => {
    setDraft((prev) => ({ ...prev, [field.key]: field.default }));
    if (field.source === 'settings') {
      setReset((prev) => (prev.includes(field.key) ? prev : [...prev, field.key]));
    }
    setSaved(false);
  };

  const dirty = catalog
    ? catalog.sections.flatMap((section) =>
        section.settings.filter((field) => isDirty(field, draft[field.key])),
      )
    : [];
  const resetOnly = reset.filter((key) => !dirty.some((field) => field.key === key));
  const changesCount = dirty.filter((field) => !reset.includes(field.key)).length + resetOnly.length;

  const save = async () => {
    if (!catalog) return;
    setBusy(true);
    setError('');
    setSaved(false);
    try {
      const values: Record<string, unknown> = {};
      for (const field of dirty) {
        if (reset.includes(field.key)) continue;
        const value = draft[field.key];
        values[field.key] = field.kind === 'int' ? Number(value) : value;
      }
      const next = await api.updateSettings({
        version: catalog.version,
        values,
        reset: resetOnly,
      });
      applyCatalog(next);
      setSaved(true);
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setConflict(true);
        setError('Настройки уже изменил другой администратор. Загрузите актуальные значения.');
      } else {
        setError(err instanceof ApiError ? err.message : 'не удалось сохранить настройки');
      }
    } finally {
      setBusy(false);
    }
  };

  if (loading) return <Loader size="l" />;
  if (!catalog) {
    return (
      <div className="stack">
        {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
      </div>
    );
  }

  const renderField = (field: SettingField) => {
    if (field.secret) {
      return (
        <label className="field" key={field.key}>
          <span>{field.title}</span>
          <Text className="muted">{field.secretSet ? 'задан' : 'не задан'}</Text>
          <Text className="muted">Секрет меняется только через env.</Text>
        </label>
      );
    }
    if (field.status === 'readonly') {
      return (
        <label className="field" key={field.key}>
          <span>
            {field.title} <Label size="s">только чтение</Label>
          </span>
          <Text>{valueText(field, field.value)}</Text>
          <Text className="muted">{field.description}</Text>
        </label>
      );
    }
    if (field.status === 'planned') {
      return (
        <label className="field" key={field.key}>
          <span>
            {field.title} <Label theme="unknown" size="s">{field.plannedRelease}</Label>
          </span>
          <TextInput value={valueText(field, field.default)} disabled onUpdate={() => {}} />
          <Text className="muted">{field.description || 'Появится в следующем релизе.'}</Text>
        </label>
      );
    }

    const value = draft[field.key];
    return (
      <label className="field" key={field.key}>
        <span>
          {field.title}
          {field.restartRequired && (
            <Label theme="warning" size="s">
              после перезапуска
            </Label>
          )}
        </span>
        {field.kind === 'bool' ? (
          <span className="check">
            <input
              type="checkbox"
              checked={Boolean(value)}
              onChange={(event) => update(field.key, event.target.checked)}
            />
            включено
          </span>
        ) : field.kind === 'enum' ? (
          <select value={String(value ?? '')} onChange={(event) => update(field.key, event.target.value)}>
            {(field.options ?? []).map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        ) : (
          <TextInput
            value={String(value ?? '')}
            placeholder={`по умолчанию: ${valueText(field, field.default)}`}
            onUpdate={(next) => update(field.key, next)}
          />
        )}
        <span className="row">
          <Text className="muted">{field.description}</Text>
          {(field.source === 'settings' || isDirty(field, value)) && (
            <Button view="flat" size="s" onClick={() => resetField(field)}>
              по умолчанию
            </Button>
          )}
        </span>
        {field.min !== undefined && field.max !== undefined && (
          <Text className="muted">
            от {field.min} до {field.max}
          </Text>
        )}
        {field.minDuration && field.maxDuration && (
          <Text className="muted">
            от {field.minDuration} до {field.maxDuration}
          </Text>
        )}
      </label>
    );
  };

  return (
    <div className="stack">
      <div className="card-head">
        <Text variant="header-2">Настройки</Text>
        <div style={{ flex: 1 }} />
        <Text className="muted">версия {catalog.version}</Text>
      </div>
      {error && (
        <Alert
          theme={conflict ? 'warning' : 'danger'}
          message={error}
          onClose={() => setError('')}
        />
      )}
      {conflict && (
        <div>
          <Button view="action" onClick={() => void load()}>
            Загрузить актуальные
          </Button>
        </div>
      )}
      {saved && <Text color="positive">сохранено</Text>}

      {catalog.sections.map((section) => (
        <div className="stack" key={section.id}>
          <Card view="filled" style={{ padding: 16 }}>
            <div className="stack">
              <Text variant="header-1">{section.title}</Text>
              <Text className="muted">{section.description}</Text>
              <div className="form-grid">{section.settings.map(renderField)}</div>
            </div>
          </Card>
          {section.id === 'links' && <TemplatesPanel />}
          {section.id === 'backups' && <BackupsPanel />}
          {section.id === 'infrastructure' && (
            <>
              <SystemPanel />
              <HistoryPanel />
            </>
          )}
        </div>
      ))}

      <div className="settings-save">
        <Text className="muted">
          {changesCount > 0 ? `${changesCount} изменений` : 'нет изменений'}
        </Text>
        <Button
          view="action"
          loading={busy}
          disabled={changesCount === 0}
          onClick={() => void save()}
        >
          Сохранить настройки
        </Button>
      </div>
    </div>
  );
}
