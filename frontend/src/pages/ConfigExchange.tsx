import { useEffect, useMemo, useState } from 'react';
import { Alert, Button, Card, Checkbox, Label, Select, Text, TextArea, TextInput } from '@gravity-ui/uikit';
import {
  api,
  ApiError,
  type ConfigBlock,
  type ConfigBlockID,
  type ConfigBlockResult,
  type ConfigPlan,
} from '../api';

const presets: Array<{ id: string; label: string; blocks: ConfigBlockID[] }> = [
  { id: 'settings', label: 'Только настройки', blocks: ['settings', 'limitTemplates'] },
  { id: 'clients-free', label: 'Без клиентов', blocks: ['settings', 'limitTemplates', 'proxy', 'users', 'services'] },
  {
    id: 'full',
    label: 'Полная выгрузка',
    blocks: ['settings', 'limitTemplates', 'proxy', 'users', 'services', 'clients'],
  },
];

const allBlocks: ConfigBlockID[] = [
  'settings',
  'limitTemplates',
  'proxy',
  'users',
  'services',
  'clients',
];

function download(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = name;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

export default function ConfigExchange() {
  const [blocks, setBlocks] = useState<ConfigBlock[]>([]);
  const [selected, setSelected] = useState<ConfigBlockID[]>(['settings', 'limitTemplates']);
  const [secrets, setSecrets] = useState(false);
  const [password, setPassword] = useState('');
  const [code, setCode] = useState('');
  const [yaml, setYaml] = useState('');
  const [fileName, setFileName] = useState('uploaded.tgadmin.yaml');
  const [plan, setPlan] = useState<ConfigPlan | null>(null);
  const [planBlocks, setPlanBlocks] = useState<ConfigBlockID[]>([]);
  const [modes, setModes] = useState<Record<string, string>>({});
  const [conflicts, setConflicts] = useState<Record<string, Record<string, string>>>({});
  const [deleteMissing, setDeleteMissing] = useState(false);
  const [confirmName, setConfirmName] = useState('');
  const [results, setResults] = useState<ConfigBlockResult[]>([]);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api
      .configBlocks()
      .then((list) => setBlocks(Array.isArray(list) ? list : []))
      .catch(() => setBlocks(allBlocks.map((id) => ({ id, title: id }))));
  }, []);

  const selectedSet = useMemo(() => new Set(selected), [selected]);

  const toggle = (id: ConfigBlockID, checked: boolean) => {
    setSelected((prev) => (checked ? [...new Set([...prev, id])] : prev.filter((item) => item !== id)));
  };

  const exportYaml = async () => {
    setBusy(true);
    setError('');
    try {
      const blob = await api.exportConfig({ blocks: selected, secrets, password, code });
      download(blob, `tgadmin-${new Date().toISOString().slice(0, 16).replace(/[:T]/g, '')}.tgadmin.yaml`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось выгрузить конфигурацию');
    } finally {
      setBusy(false);
    }
  };

  const precheck = async () => {
    setBusy(true);
    setError('');
    setPlan(null);
    setResults([]);
    try {
      const next = await api.configPrecheck(yaml, fileName, password, code);
      setPlan(next);
      setPlanBlocks(next.blocks.map((block) => block.id));
      const nextConflicts: Record<string, Record<string, string>> = {};
      for (const [block, items] of Object.entries(next.conflicts ?? {})) {
        nextConflicts[block] = {};
        for (const item of items) nextConflicts[block][item.name] = item.action;
      }
      setConflicts(nextConflicts);
      setModes({ settings: 'merge', proxy: 'merge' });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось проверить файл');
    } finally {
      setBusy(false);
    }
  };

  const apply = async () => {
    if (!plan) return;
    setBusy(true);
    setError('');
    try {
      const out = await api.configApply({
        planId: plan.id,
        blocks: planBlocks,
        modes,
        conflicts,
        deleteMissing,
        confirmName,
      });
      setResults(out.results);
      if (out.error) setError(out.error);
      setPlan(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось применить конфигурацию');
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card view="filled" style={{ padding: 16 }}>
      <div className="stack">
        <Text variant="header-1">Обмен конфигурацией</Text>
        <Text className="muted">
          YAML для переноса настроек между установками. Секреты по умолчанию не выгружаются.
        </Text>
        {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}

        <div className="row">
          {presets.map((preset) => (
            <Button key={preset.id} view="outlined" onClick={() => setSelected(preset.blocks)}>
              {preset.label}
            </Button>
          ))}
        </div>
        <div className="row">
          {(blocks.length > 0 ? blocks : allBlocks.map((id) => ({ id, title: id }))).map((block) => (
            <Checkbox
              key={block.id}
              checked={selectedSet.has(block.id)}
              onUpdate={(checked) => toggle(block.id, checked)}
              content={block.title}
              controlProps={{ 'aria-label': block.title }}
            />
          ))}
        </div>
        <Checkbox
          checked={secrets}
          onUpdate={setSecrets}
          content="Включить секреты"
          controlProps={{ 'aria-label': 'Включить секреты' }}
        />
        {secrets && (
          <div className="row">
            <TextInput
              type="password"
              placeholder="Текущий пароль"
              value={password}
              onUpdate={setPassword}
              controlProps={{ 'aria-label': 'Пароль для секретов' }}
            />
            <TextInput
              placeholder="Код TOTP, если включён"
              value={code}
              onUpdate={setCode}
              controlProps={{ 'aria-label': 'Код для секретов' }}
            />
          </div>
        )}
        <div>
          <Button view="action" loading={busy} disabled={selected.length === 0} onClick={() => void exportYaml()}>
            Скачать YAML
          </Button>
        </div>

        <Text variant="header-2">Импорт</Text>
        <input
          type="file"
          accept=".yaml,.yml,text/yaml"
          aria-label="YAML"
          onChange={(event) => {
            const file = event.target.files?.[0];
            if (!file) return;
            setFileName(file.name);
            void file.text().then(setYaml);
          }}
        />
        <TextArea
          rows={6}
          placeholder="Или вставьте YAML"
          value={yaml}
          onUpdate={setYaml}
          controlProps={{ 'aria-label': 'Текст YAML' }}
        />
        <div>
          <Button view="outlined" loading={busy} disabled={!yaml.trim()} onClick={() => void precheck()}>
            Проверить
          </Button>
        </div>

        {plan && (
          <div className="stack">
            <Text variant="header-2">Что изменится</Text>
            <div className="row">
              {plan.blocks.map((block) => (
                <Checkbox
                  key={block.id}
                  checked={planBlocks.includes(block.id)}
                  onUpdate={(checked) =>
                    setPlanBlocks((prev) =>
                      checked ? [...new Set([...prev, block.id])] : prev.filter((id) => id !== block.id),
                    )
                  }
                  content={`${block.title}: ${block.changes}`}
                  controlProps={{ 'aria-label': block.title }}
                />
              ))}
            </div>
            {(['settings', 'proxy'] as const).map((block) =>
              planBlocks.includes(block) ? (
                <div className="row" key={block}>
                  <Text>{block === 'settings' ? 'Настройки' : 'Прокси'}</Text>
                  <Select
                    aria-label={`Режим ${block}`}
                    value={[modes[block] ?? 'merge']}
                    options={[
                      { value: 'merge', content: 'слияние' },
                      { value: 'replace', content: 'замена' },
                    ]}
                    onUpdate={([value]) => setModes((prev) => ({ ...prev, [block]: value ?? 'merge' }))}
                  />
                </div>
              ) : null,
            )}

            {Object.entries(plan.diff ?? {}).map(([block, entries]) =>
              entries.length === 0 ? null : (
                <div className="stack" key={block}>
                  <Text variant="subheader-1">{block}</Text>
                  {entries.slice(0, 50).map((entry) => (
                    <Text key={`${block}-${entry.path}`} className="muted">
                      {entry.path}: {entry.secret ? 'секрет будет изменён' : `${String(entry.before ?? '—')} → ${String(entry.after ?? '—')}`}
                    </Text>
                  ))}
                </div>
              ),
            )}

            {Object.entries(plan.conflicts ?? {}).map(([block, items]) =>
              items.length === 0 ? null : (
                <div className="stack" key={`conflict-${block}`}>
                  <Text variant="subheader-1">Конфликты: {block}</Text>
                  {items.map((item) => (
                    <div className="row" key={`${block}-${item.name}`}>
                      <Text>{item.name}</Text>
                      <Text className="muted">{item.changes.join(', ')}</Text>
                      <div style={{ flex: 1 }} />
                      <Button
                        view={conflicts[block]?.[item.name] === 'overwrite' ? 'action' : 'outlined'}
                        onClick={() => setConflicts((prev) => ({ ...prev, [block]: { ...prev[block], [item.name]: 'overwrite' } }))}
                      >
                        переписать {item.name}
                      </Button>
                      <Button
                        view={conflicts[block]?.[item.name] !== 'overwrite' ? 'action' : 'outlined'}
                        onClick={() => setConflicts((prev) => ({ ...prev, [block]: { ...prev[block], [item.name]: 'keep' } }))}
                      >
                        оставить {item.name}
                      </Button>
                    </div>
                  ))}
                </div>
              ),
            )}

            <Checkbox
              checked={deleteMissing}
              onUpdate={setDeleteMissing}
              content="Удалить отсутствующие в файле"
              controlProps={{ 'aria-label': 'Удалить отсутствующие' }}
            />
            {deleteMissing && (
              <>
                <Text className="muted">Введите имя установки из metadata.name для подтверждения.</Text>
                <TextInput
                  placeholder="Имя установки"
                  value={confirmName}
                  onUpdate={setConfirmName}
                  controlProps={{ 'aria-label': 'Имя установки' }}
                />
              </>
            )}
            <div>
              <Button
                view="action"
                loading={busy}
                disabled={planBlocks.length === 0 || (deleteMissing && !confirmName)}
                onClick={() => void apply()}
              >
                Применить выбранное
              </Button>
            </div>
          </div>
        )}

        {results.length > 0 && (
          <div className="stack">
            <Text variant="header-2">Результат</Text>
            {results.map((result) => (
              <div className="stack" key={result.block}>
                <div className="row">
                  <Text>{result.block}</Text>
                  <Label theme={result.status === 'applied' ? 'success' : 'danger'}>{result.status}</Label>
                  {result.error && <Text className="muted">{result.error}</Text>}
                </div>
                {result.passwords &&
                  Object.entries(result.passwords).map(([name, value]) => (
                    <Text key={name} className="muted">
                      {name}: временный пароль {value}
                    </Text>
                  ))}
                {result.links &&
                  Object.entries(result.links).map(([name, links]) => (
                    <Text key={name} className="muted">
                      {name}: {links[0]}
                    </Text>
                  ))}
              </div>
            ))}
          </div>
        )}
      </div>
    </Card>
  );
}
