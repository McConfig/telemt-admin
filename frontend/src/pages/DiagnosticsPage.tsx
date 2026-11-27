import { useCallback, useEffect, useMemo, useState } from 'react';
import { Alert, Button, Card, Label, Loader, Text } from '@gravity-ui/uikit';
import { Table, useTable } from '@gravity-ui/table';
import type { ColumnDef } from '@gravity-ui/table/tanstack';
import { api, ApiError, type Diagnostics } from '../api';
import { formatTime } from '../format';

const errorHints: [string, string][] = [
  ['telemt_unreachable', 'проверьте адрес, порт и доступность Control API'],
  ['telemt_timeout', 'проверьте нагрузку и время ответа Control API'],
  ['telemt_dns_failed', 'проверьте DNS и имя хоста Control API'],
  ['telemt_tls_failed', 'проверьте сертификат и цепочку доверия'],
  ['telemt_auth_failed', 'проверьте TELEMT_API_TOKEN и авторизацию'],
  ['telemt_not_found', 'проверьте имя клиента и версию Control API'],
  ['telemt_conflict', 'состояние изменилось, обновите данные'],
  ['telemt_bad_request', 'проверьте поля и ограничения telemt'],
  ['telemt_config_read_only', 'конфигурация telemt смонтирована только для чтения'],
  ['telemt_config_permission_denied', 'проверьте владельца и права каталога конфигурации'],
  ['telemt_config_storage_full', 'проверьте свободное место на диске'],
  ['telemt_config_write_failed', 'проверьте каталог, монтирование и журнал telemt'],
  ['telemt_config_read_failed', 'проверьте основной файл, include-файлы и права'],
  ['telemt_invalid_response', 'ответ повреждён или несовместим по версии'],
  ['telemt_api_error', 'смотрите журнал telemt и номер запроса'],
  ['telemt_unavailable', 'Control API временно недоступен'],
];

function Stat({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="stat-tile">
      <span className="stat-label">{label}</span>
      <span className="stat-value">{value}</span>
    </div>
  );
}

export default function DiagnosticsPage() {
  const [data, setData] = useState<Diagnostics | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      setData(await api.diagnostics());
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось получить диагностику');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  if (loading) return <Loader size="l" />;

  return (
    <div className="stack">
      <div className="card-head">
        <Text variant="header-2">Диагностика</Text>
        <div style={{ flex: 1 }} />
        <Button view="outlined" onClick={() => void load()}>
          Обновить
        </Button>
      </div>
      {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
      {!data ? null : (
        <>
          <Alert
            theme={data.status === 'error' ? 'danger' : data.status === 'warn' ? 'warning' : 'success'}
            title={
              data.status === 'error'
                ? 'Есть критическая проблема'
                : data.status === 'warn'
                  ? 'Есть предупреждения'
                  : 'Всё в порядке'
            }
            message={
              data.warnings.length > 0
                ? data.warnings.join('; ')
                : `Проверено ${formatTime(data.checkedAt)}`
            }
          />

          <Card view="filled" style={{ padding: 16 }}>
            <div className="stack">
              <Text variant="header-1">Control API</Text>
              <div className="stat-grid">
                <Stat label="Доступен" value={data.telemt.ok ? 'да' : 'нет'} />
                <Stat label="Задержка" value={`${data.telemt.latencyMs} мс`} />
                <Stat label="Версия" value={data.telemt.version ?? '—'} />
              </div>
              {data.telemt.error && (
                <Alert
                  theme="danger"
                  title={data.telemt.code ?? 'ошибка'}
                  message={`${data.telemt.error}${data.telemt.hint ? ` — ${data.telemt.hint}` : ''}`}
                />
              )}
            </div>
          </Card>

          <Card view="filled" style={{ padding: 16 }}>
            <div className="stack">
              <Text variant="header-1">Сборщик и GeoIP</Text>
              <div className="stat-grid">
                <Stat label="Сборщик" value={data.collector.up ? 'работает' : 'остановлен'} />
                <Stat
                  label="Последний сбор"
                  value={data.collector.lastSuccess ? formatTime(data.collector.lastSuccess) : '—'}
                />
                <Stat label="GeoIP" value={data.geoip.loaded ? 'включён' : 'выключен'} />
                <Stat
                  label="Возраст баз"
                  value={data.geoip.built ? `${data.geoip.ageDays} дн.` : '—'}
                />
              </div>
              {data.collector.lastError && (
                <Alert theme="warning" message={data.collector.lastError} />
              )}
            </div>
          </Card>

          <Card view="filled" style={{ padding: 16 }}>
            <div className="stack">
              <Text variant="header-1">Valkey</Text>
              <div className="stat-grid">
                <Stat label="Доступен" value={data.valkey.ok ? 'да' : 'нет'} />
                <Stat label="Задержка" value={`${data.valkey.latencyMs} мс`} />
                <Stat label="Ключей" value={data.valkey.keys} />
                <Stat label="Память" value={data.valkey.memory ?? '—'} />
              </div>
              {data.valkey.error && <Alert theme="danger" message={data.valkey.error} />}
            </div>
          </Card>

          <Card view="filled" style={{ padding: 16 }}>
            <div className="stack">
              <Text variant="header-1">Вебхуки</Text>
              <div className="stat-grid">
                <Stat label="Настроены" value={data.webhooks.configured ? 'да' : 'нет'} />
                <Stat label="Секрет" value={data.webhooks.secretSet ? 'задан' : 'не задан'} />
                <Stat label="Отправлено" value={data.webhooks.sent} />
                <Stat label="Ошибок" value={data.webhooks.failed} />
              </div>
              <Text className="muted">
                События: {data.webhooks.events.length > 0 ? data.webhooks.events.join(', ') : '—'}
              </Text>
              {data.webhooks.lastAt && (
                <Text className="muted">
                  Последняя: {data.webhooks.lastEvent} · статус {data.webhooks.lastStatus || '—'} ·{' '}
                  {formatTime(data.webhooks.lastAt)}
                </Text>
              )}
              {data.webhooks.lastError && <Alert theme="warning" message={data.webhooks.lastError} />}
            </div>
          </Card>

          <Card view="filled" style={{ padding: 16 }}>
            <div className="stack">
              <Text variant="header-1">Бэкапы</Text>
              <div className="stat-grid">
                <Stat
                  label="Снимок Valkey"
                  value={data.backups.valkey.lastSave ? formatTime(data.backups.valkey.lastSave) : '—'}
                />
                <Stat label="Статус" value={data.backups.valkey.lastStatus || '—'} />
                <Stat label="Ключей" value={data.backups.valkey.keys} />
                <Stat
                  label="Хостовый снимок"
                  value={
                    data.backups.host.available && data.backups.host.lastRun
                      ? formatTime(data.backups.host.lastRun)
                      : '—'
                  }
                />
                <Stat label="Хранить" value={`${data.backups.host.keepDays ?? '—'} дн.`} />
                <Stat
                  label="Восстановление"
                  value={data.backups.host.available ? (data.backups.host.restoreTest ? 'да' : 'нет') : '—'}
                />
              </div>
            </div>
          </Card>

          <Card view="filled" style={{ padding: 16 }}>
            <div className="stack">
              <Text variant="header-1">Безопасность</Text>
              <div className="stat-grid">
                <Stat label="Обязательная 2FA" value={data.security.require2fa ? 'да' : 'нет'} />
                <Stat label="Ключ TOTP" value={data.security.totpKeySet ? 'задан' : 'не задан'} />
                <Stat
                  label="Льготный срок"
                  value={data.security.require2fa ? `${data.security.daysLeft} дн.` : '—'}
                />
              </div>
              {data.security.adminsWithoutTotp.length > 0 ? (
                <Text className="muted">
                  Админы без 2FA: {data.security.adminsWithoutTotp.join(', ')}
                </Text>
              ) : (
                <Text className="muted">Все администраторы с 2FA.</Text>
              )}
              {data.security.deadline && (
                <Text className="muted">Дедлайн: {formatTime(data.security.deadline)}</Text>
              )}
            </div>
          </Card>

          <Card view="filled" style={{ padding: 16 }}>
            <div className="stack">
              <Text variant="header-1">Коды ошибок Control API</Text>
              <ErrorHintsTable hints={errorHints} activeCode={data.telemt.code} />
            </div>
          </Card>
        </>
      )}
    </div>
  );
}

function ErrorHintsTable({
  hints,
  activeCode,
}: {
  hints: [string, string][];
  activeCode?: string;
}) {
  const columns = useMemo<ColumnDef<[string, string]>[]>(
    () => [
      {
        id: 'code',
        header: 'Код',
        cell: ({ row }) => (
          <Label size="s" theme={activeCode === row.original[0] ? 'danger' : 'unknown'}>
            {row.original[0]}
          </Label>
        ),
      },
      { id: 'hint', header: 'Что проверить', cell: ({ row }) => row.original[1] },
    ],
    [activeCode],
  );
  const table = useTable({ columns, data: hints });
  return (
    <div className="table-scroll">
      <Table table={table} />
    </div>
  );
}
