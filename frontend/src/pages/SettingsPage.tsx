import { useCallback, useEffect, useState } from 'react';
import { Alert, Button, Card, Checkbox, Label, Loader, Select, Text, TextInput } from '@gravity-ui/uikit';
import { api, ApiError, type SettingField, type SettingsCatalog } from '../api';
import { BackupsPanel, HistoryPanel, SystemPanel, TemplatesPanel } from './SettingsExtras';
import ConfigExchange from './ConfigExchange';
import ProxyPanel from './ProxyPanel';

function isDirty(field: SettingField, value: unknown): boolean {
  if (field.status !== 'active') return false;
  if (field.kind === 'bool') return value !== field.value;
  return String(value ?? '') !== String(field.value ?? '');
}

const tabs = [
  { id: 'risk', label: 'Риск', sections: ['risk'] },
  { id: 'collect', label: 'Сбор и данные', sections: ['collect'] },
  { id: 'quotas', label: 'Квоты', sections: ['quotas'] },
  { id: 'links', label: 'Ссылки', sections: ['links'] },
  { id: 'templates', label: 'Шаблоны', sections: [] },
  { id: 'security', label: 'Безопасность', sections: ['security', 'interface', 'logging', 'notifications'] },
  { id: 'backups', label: 'Бэкапы', sections: ['backups'] },
  { id: 'proxy', label: 'Прокси', sections: [] },
  { id: 'system', label: 'Система', sections: [] },
  { id: 'infrastructure', label: 'Инфраструктура', sections: ['infrastructure'] },
  { id: 'history', label: 'История', sections: [] },
] as const;

type TabID = (typeof tabs)[number]['id'];

function valueText(field: SettingField, value: unknown): string {
  if (value === undefined || value === null || value === '') return '—';
  if (field.kind === 'bool') return value ? 'да' : 'нет';
  return String(value);
}

export default function SettingsPage() {
  const [catalog, setCatalog] = useState<SettingsCatalog | null>(null);
  const [tab, setTab] = useState<TabID>('risk');
  const [draft, setDraft] = useState<Record<string, unknown>>({});
  const [reset, setReset] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [saved, setSaved] = useState(false);
  const [conflict, setConflict] = useState(false);
  const [webhookBusy, setWebhookBusy] = useState(false);
  const [webhookMessage, setWebhookMessage] = useState('');

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

  const activeTab = tabs.find((item) => item.id === tab) ?? tabs[0];
  const dirtyKeys = new Set([...dirty.map((field) => field.key), ...resetOnly]);
  const tabHasChanges = (id: TabID) => {
    const item = tabs.find((entry) => entry.id === id);
    if (!item || !catalog) return false;
    return item.sections.some((sectionID) =>
      catalog.sections
        .find((section) => section.id === sectionID)
        ?.settings.some((field) => dirtyKeys.has(field.key)),
    );
  };

  const testWebhook = async () => {
    setWebhookBusy(true);
    setWebhookMessage('');
    try {
      const result = await api.webhookTest();
      setWebhookMessage(`Отправлено, статус ${result.status}`);
    } catch (err) {
      setWebhookMessage(err instanceof ApiError ? err.message : 'вебхук не отправлен');
    } finally {
      setWebhookBusy(false);
    }
  };

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
      if (field.status === 'active') {
        const value = draft[field.key];
        return (
          <label className="field" key={field.key}>
            <span>{field.title}</span>
            <TextInput
              type="password"
              value={String(value ?? '')}
              placeholder={field.secretSet ? 'задан, оставьте пустым' : 'не задан'}
              onUpdate={(next) => update(field.key, next)}
            />
            <Text className="muted">{field.description}</Text>
            {(field.source === 'settings' || isDirty(field, value)) && (
              <Button view="flat" size="s" onClick={() => resetField(field)}>
                сбросить
              </Button>
            )}
          </label>
        );
      }
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
          <Checkbox
            controlProps={{ 'aria-label': field.title }}
            checked={Boolean(value)}
            onUpdate={(checked) => update(field.key, checked)}
            content="включено"
          />
        ) : field.kind === 'enum' ? (
          <Select
            aria-label={field.title}
            value={[String(value ?? '')]}
            options={(field.options ?? []).map((option) => ({
              value: option.value,
              content: option.label,
            }))}
            onUpdate={([next]) => update(field.key, next ?? '')}
            width="max"
          />
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

      <div className="tabs">
        {tabs.map((item) => (
          <Button
            key={item.id}
            view={tab === item.id ? 'action' : 'flat'}
            onClick={() => setTab(item.id)}
          >
            {item.label}
            {tabHasChanges(item.id) ? ' ·' : ''}
          </Button>
        ))}
      </div>

      {activeTab.sections.map((sectionID) => {
        const section = catalog.sections.find((item) => item.id === sectionID);
        if (!section) return null;
        return (
          <Card view="filled" style={{ padding: 16 }} key={section.id}>
            <div className="stack">
              <Text variant="header-1">{section.title}</Text>
              <Text className="muted">{section.description}</Text>
              <div className="form-grid">{section.settings.map(renderField)}</div>
              {section.id === 'notifications' && (
                <div className="row">
                  <Button view="outlined" loading={webhookBusy} onClick={() => void testWebhook()}>
                    Проверить вебхук
                  </Button>
                  {webhookMessage && <Text className="muted">{webhookMessage}</Text>}
                </div>
              )}
              {section.id === 'collect' && (
                <Text className="muted">
                  Интервал применяется со следующего цикла. Увеличение retention не возвращает
                  данные, которые уже удалены по старому сроку.
                </Text>
              )}
            </div>
          </Card>
        );
      })}
      {tab === 'templates' && <TemplatesPanel />}
      {tab === 'proxy' && <ProxyPanel />}
      {tab === 'backups' && (
        <>
          <ConfigExchange />
          <BackupsPanel />
        </>
      )}
      {tab === 'system' && <SystemPanel />}
      {tab === 'history' && <HistoryPanel />}

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
