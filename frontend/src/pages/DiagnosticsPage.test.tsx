import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import DiagnosticsPage from './DiagnosticsPage';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';

const diagnostics = {
  status: 'warn',
  checkedAt: new Date().toISOString(),
  version: 'v1.5.1',
  uptime: '1h',
  telemt: {
    url: 'http://telemt:9091',
    ok: false,
    latencyMs: 5,
    code: 'telemt_timeout',
    error: 'Control API не ответил вовремя',
    hint: 'проверьте нагрузку',
  },
  collector: { up: true, lastSuccess: new Date().toISOString() },
  geoip: { loaded: true, built: new Date().toISOString(), ageDays: 1 },
  valkey: { ok: true, latencyMs: 1, keys: 100, memory: '1M' },
  webhooks: {
    configured: true,
    events: ['user.create'],
    secretSet: false,
    sent: 2,
    failed: 1,
    lastError: 'вебхук ответил 500',
  },
  backups: {
    valkey: { saveInProgress: false, lastStatus: 'ok', keys: 100, memory: '1M' },
    host: { available: false },
  },
  security: {
    require2fa: true,
    graceDays: 7,
    daysLeft: 2,
    totpKeySet: true,
    adminsWithoutTotp: ['admin'],
  },
  warnings: ['Control API не ответил вовремя', 'вебхук настроен без секрета подписи'],
};

describe('DiagnosticsPage', () => {
  it('показывает проблемы, коды и подсказки', async () => {
    stubFetch({ '/api/diagnostics': diagnostics });
    renderUI(<DiagnosticsPage />);
    expect(await screen.findByText('Диагностика')).toBeInTheDocument();
    expect(screen.getByText(/Есть предупреждения/)).toBeInTheDocument();
    expect(screen.getAllByText(/Control API не ответил вовремя/).length).toBeGreaterThan(0);
    expect(screen.getAllByText('telemt_timeout').length).toBeGreaterThan(0);
    expect(screen.getByText('вебхук ответил 500')).toBeInTheDocument();
    expect(screen.getByText(/admin/)).toBeInTheDocument();
    expect(screen.getByText('проверьте адрес, порт и доступность Control API')).toBeInTheDocument();
  });
});
