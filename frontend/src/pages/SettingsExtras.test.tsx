import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { BackupsPanel, HistoryPanel, SystemPanel, TemplatesPanel } from './SettingsExtras';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';

const template = {
  name: 'router',
  maxIps: 5,
  maxTcpConns: 20,
  dataQuotaBytes: 10 * 1024 ** 3,
  quotaPeriod: 'monthly',
  riskWarnIps: 4,
  riskHighIps: 6,
  riskDistinct7d: 20,
  riskNewCountries: false,
};

function calls(method: string, part: string) {
  return vi
    .mocked(fetch)
    .mock.calls.filter(
      ([url, init]) =>
        String(url).includes(part) && (init as RequestInit | undefined)?.method === method,
    );
}

describe('SettingsExtras', () => {
  it('создаёт, изменяет и удаляет шаблон', async () => {
    stubFetch({
      '/api/settings/templates': { default: 'router', templates: [template] },
      '/api/settings/templates/router': template,
    });
    renderUI(<TemplatesPanel />);
    await screen.findByText('router');

    await userEvent.click(screen.getByRole('button', { name: 'Изменить' }));
    const maxIps = screen.getByLabelText('Максимум IP');
    await userEvent.clear(maxIps);
    await userEvent.type(maxIps, '7');
    await userEvent.click(screen.getByRole('button', { name: 'Сохранить шаблон' }));
    await screen.findByText('сохранено');
    expect(JSON.parse(String((calls('PUT', '/templates/router')[0]?.[1] as RequestInit).body))).toMatchObject({
      name: 'router',
      maxIps: 7,
    });

    await userEvent.click(screen.getByRole('button', { name: 'Новый шаблон' }));
    await userEvent.type(screen.getByLabelText('Имя'), 'phone');
    await userEvent.click(screen.getByRole('button', { name: 'Создать шаблон' }));
    expect(JSON.parse(String((calls('POST', '/api/settings/templates')[0]?.[1] as RequestInit).body))).toMatchObject({
      name: 'phone',
    });

    await userEvent.click(screen.getByRole('button', { name: 'Удалить' }));
    expect(calls('DELETE', '/templates/router')).toHaveLength(1);
  });

  it('показывает статус бэкапов и запускает снимок', async () => {
    stubFetch({
      '/api/settings/backups': {
        valkey: {
          lastSave: '2026-11-25T04:30:00Z',
          saveInProgress: false,
          lastStatus: 'ok',
          keys: 120,
          memory: '1.2M',
        },
        host: { available: false },
      },
    });
    renderUI(<BackupsPanel />);
    expect(await screen.findByText('120')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Снимок сейчас' }));
    expect(calls('POST', '/api/settings/backups/snapshot')).toHaveLength(1);
    expect(await screen.findByText(/Статус хоста недоступен/)).toBeInTheDocument();
  });

  it('показывает систему и историю', async () => {
    stubFetch({
      '/api/settings/system': {
        version: 'v1.3.0',
        uptime: '2h',
        collector: { up: true },
        geoip: { loaded: true },
        valkey: { keys: 42, memory: '512K' },
        counts: { uiUsers: 2, auditEvents: 7, templates: 1 },
      },
      '/api/settings/history': [
        {
          At: '2026-11-25T10:00:00Z',
          Actor: 'admin',
          Action: 'settings.update',
          Target: 'risk.warn_ips',
          Result: '3 -> 4',
          IP: '192.0.2.1',
        },
      ],
    });
    renderUI(
      <>
        <SystemPanel />
        <HistoryPanel />
      </>,
    );
    expect(await screen.findByText('v1.3.0')).toBeInTheDocument();
    expect(screen.getByText('42')).toBeInTheDocument();
    expect(await screen.findByText('risk.warn_ips')).toBeInTheDocument();
    expect(screen.getByText('3 -> 4')).toBeInTheDocument();
  });
});
