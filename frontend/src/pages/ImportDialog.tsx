import { useEffect, useMemo, useState } from 'react';
import { Alert, Button, Dialog, Text, useFileInput } from '@gravity-ui/uikit';
import { Table, useTable } from '@gravity-ui/table';
import type { ColumnDef } from '@gravity-ui/table/tanstack';
import { api, ApiError, type ImportResult, type ImportRow } from '../api';

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

  const chooseFile = async (file: File | undefined) => {
    if (!file) return;
    setFileName(file.name);
    setResult(null);
    setError('');
    try {
      setCsv(await readTextFile(file));
    } catch {
      setCsv('');
      setError('не удалось прочитать файл');
    }
  };
  const { controlProps, triggerProps } = useFileInput({
    onUpdate: (files) => void chooseFile(files[0]),
  });

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
            <input {...controlProps} aria-label="CSV-файл" accept=".csv,text/csv" />
            <Button view="outlined" {...triggerProps}>
              Выбрать файл
            </Button>
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
            <ImportRowsTable rows={result.rows} dryRun={result.dryRun} />
          )}
        </div>
      </Dialog.Body>
      <Dialog.Footer onClickButtonCancel={onClose} textButtonCancel="Закрыть" />
    </Dialog>
  );
}

function ImportRowsTable({ rows, dryRun }: { rows: ImportRow[]; dryRun: boolean }) {
  const columns = useMemo<ColumnDef<ImportRow>[]>(
    () => [
      { id: 'line', header: 'Строка', cell: ({ row }) => row.original.line },
      { id: 'name', header: 'Имя', cell: ({ row }) => row.original.name || '—' },
      {
        id: 'status',
        header: 'Статус',
        cell: ({ row }) =>
          row.original.status === 'create'
            ? dryRun
              ? 'создать'
              : 'создано'
            : row.original.status === 'skip'
              ? 'пропуск'
              : 'ошибка',
      },
      { id: 'message', header: 'Сообщение', cell: ({ row }) => row.original.message ?? '' },
    ],
    [dryRun],
  );
  const table = useTable({ columns, data: rows });
  return (
    <div className="table-scroll">
      <Table table={table} />
    </div>
  );
}
