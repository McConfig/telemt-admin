import { useEffect, useState } from 'react';
import { Alert, Button, Dialog, Text } from '@gravity-ui/uikit';
import { api, ApiError, type ImportResult } from '../api';

const importHeader =
  'name,secret,max_unique_ips,max_tcp_conns,data_quota_bytes,quota_period,' +
  'rate_limit_up_bps,rate_limit_down_bps,expiration_rfc3339,enabled\n';

function readTextFile(file: File): Promise<string> {
  if (typeof file.text === 'function') return file.text();
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result ?? ''));
    reader.onerror = () => reject(reader.error ?? new Error('не удалось прочитать файл'));
    reader.readAsText(file);
  });
}

export default function ImportDialog({
  open,
  onClose,
  onImported,
}: {
  open: boolean;
  onClose: () => void;
  onImported: () => void;
}) {
  const [fileName, setFileName] = useState('');
  const [csv, setCsv] = useState('');
  const [result, setResult] = useState<ImportResult | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!open) return;
    setFileName('');
    setCsv('');
    setResult(null);
    setError('');
    setBusy(false);
  }, [open]);

  const chooseFile = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;
    setFileName(file.name);
    setResult(null);
    setError('');
    try {
      setCsv(await readTextFile(file));
    } catch {
      setCsv('');
      setError('не удалось прочитать файл');
    } finally {
      event.target.value = '';
    }
  };

  const check = async () => {
    setBusy(true);
    setError('');
    try {
      setResult(await api.importUsers(csv, true));
    } catch (err) {
      setResult(null);
      setError(err instanceof ApiError ? err.message : 'не удалось проверить CSV');
    } finally {
      setBusy(false);
    }
  };

  const apply = async () => {
    setBusy(true);
    setError('');
    try {
      setResult(await api.importUsers(csv, false));
      onImported();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось импортировать клиентов');
    } finally {
      setBusy(false);
    }
  };

  const downloadTemplate = () => {
    const sample =
      importHeader +
      'example-user,,5,10,10737418240,monthly,,,2026-12-31T23:59:59Z,true\n';
    const url = URL.createObjectURL(new Blob([sample], { type: 'text/csv;charset=utf-8' }));
    const link = document.createElement('a');
    link.href = url;
    link.download = 'telemt-clients.csv';
    link.click();
    URL.revokeObjectURL(url);
  };

  const summary = result
    ? result.dryRun
      ? `Всего: ${result.total}, будет создано: ${result.create}, пропущено: ${result.skip}, ошибок: ${result.error}`
      : `Создано: ${result.create}, пропущено: ${result.skip}, ошибок: ${result.error}`
    : '';

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()} maxWidth="l">
      <Dialog.Header caption="Импорт клиентов" />
      <Dialog.Body>
        <div className="stack">
          {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
          <Text className="muted">
            CSV с заголовком, до 1 МБ и 1000 строк. Обязательна только колонка name.
            Поддерживаются запятая и точка с запятой, пустые необязательные поля
            означают «не задано».
          </Text>
          <div className="row">
            <label className="field">
              <span>Файл CSV</span>
              <input
                type="file"
                aria-label="CSV-файл"
                accept=".csv,text/csv"
                onChange={(event) => void chooseFile(event)}
              />
            </label>
            <Button view="outlined" onClick={downloadTemplate}>
              Скачать шаблон
            </Button>
          </div>
          {fileName && <Text>Выбран файл: {fileName}</Text>}
          <div className="row">
            <Button view="outlined" disabled={!csv || busy} loading={busy} onClick={() => void check()}>
              Проверить
            </Button>
            {result?.dryRun && (
              <Button
                view="action"
                disabled={result.create === 0 || busy}
                loading={busy}
                onClick={() => void apply()}
              >
                Создать {result.create}
              </Button>
            )}
          </div>
          {summary && <Text>{summary}</Text>}
          {result && result.rows.length > 0 && (
            <div className="table-scroll">
              <table>
                <thead>
                  <tr>
                    <th>Строка</th>
                    <th>Имя</th>
                    <th>Статус</th>
                    <th>Сообщение</th>
                  </tr>
                </thead>
                <tbody>
                  {result.rows.map((row) => (
                    <tr key={`${row.line}-${row.name}`}>
                      <td>{row.line}</td>
                      <td>{row.name || '—'}</td>
                      <td>
                        {row.status === 'create'
                          ? result.dryRun
                            ? 'создать'
                            : 'создано'
                          : row.status === 'skip'
                            ? 'пропуск'
                            : 'ошибка'}
                      </td>
                      <td>{row.message ?? ''}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </Dialog.Body>
      <Dialog.Footer onClickButtonCancel={onClose} textButtonCancel="Закрыть" />
    </Dialog>
  );
}
