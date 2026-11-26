import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { BackupsPanel, HistoryPanel, SystemPanel, TemplatesPanel } from './SettingsExtras';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';

const router = {
  name: 'router',
  maxIps: 5,
  maxTcpConns: 20,
  dataQuotaBytes: 10 * 1024 ** 3,
  quotaPeriod: 'monthly',
  expirationDays: 7,
  rateLimitUpBps: 1000,
  rateLimitDownBps: 2000,
  riskWarnIps: 4,
  riskHighIps: 6,
  riskDistinct7d: 20,
  riskNewCountries: false,
  sourceDeny: ['203.0.113.0/24'],
};

const phone = { name: 'phone', maxIps: 2 };

function calls(method: string, part: string) {
  return vi
    .mocked(fetch)
    .mock.calls.filter(
      ([url, init]) =>
        String(url).includes(part) && (init as RequestInit | undefined)?.method === method,
    );
}

describe('SettingsExtras — расширенные сценарии', () => {
  it('назначает шаблон по умолчанию и показывает ошибки удаления', async () => {
    stubFetch({
      '/api/settings/templates': (_url: URL, init?: RequestInit) => {
        if (init?.method === 'DELETE') {
          return new Response(JSON.stringify({ error: { code: 'x', message: 'шаблон используется' } }), {
            status: 409,
            headers: { 'Content-Type': 'application/json' },
          });
        }
        return { default: 'router', templates: [router, phone] };
      },
      '/api/settings/templates/default': { default: 'phone' },
    });
    renderUI(<TemplatesPanel />);
    await screen.findByText('phone');

    await userEvent.click(screen.getByRole('button', { name: 'По умолчанию' }));
    await waitFor(() =>
      expect(
        calls('PUT', '/api/settings/templates/default').length,
      ).toBe(1),
    );
    expect(
      JSON.parse(String((calls('PUT', '/api/settings/templates/default')[0][1] as RequestInit).body)),
    ).toMatchObject({ name: 'phone' });

    await userEvent.click(screen.getAllByRole('button', { name: 'Удалить' })[1]);
    expect(await screen.findByText('шаблон используется')).toBeInTheDocument();
  });

  it('показывает пустой список шаблонов и ошибки без ApiError', async () => {
    stubFetch({
      '/api/settings/templates': (_url: URL, init?: RequestInit) => {
        if (init?.method === 'POST') throw new Error('сеть недоступна');
        return { default: '', templates: [] };
      },
    });
    renderUI(<TemplatesPanel />);
    expect(await screen.findByText('Шаблонов пока нет.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Создать шаблон' })).toBeDisabled();

    await userEvent.click(screen.getByRole('button', { name: 'Новый шаблон' }));
    await userEvent.type(screen.getByLabelText('Имя'), 'tmp');
    await userEvent.click(screen.getByRole('button', { name: 'Создать шаблон' }));
    expect(await screen.findByText('не удалось сохранить шаблон')).toBeInTheDocument();
  });

  it('показывает ошибку удаления без ApiError', async () => {
    stubFetch({
      '/api/settings/templates': (_url: URL, init?: RequestInit) => {
        if (init?.method === 'DELETE') throw new Error('сеть недоступна');
        return { default: 'router', templates: [router] };
      },
    });
    renderUI(<TemplatesPanel />);
    await screen.findByText('router');
    await userEvent.click(screen.getByRole('button', { name: 'Удалить' }));
    expect(await screen.findByText('не удалось удалить шаблон')).toBeInTheDocument();
  });

  it('открывает редактирование шаблона и отменяет его', async () => {
    stubFetch({
      '/api/settings/templates': { default: 'router', templates: [router] },
    });
    renderUI(<TemplatesPanel />);
    await screen.findByText('router');
    await userEvent.click(screen.getByRole('button', { name: 'Изменить' }));
    expect(screen.getByText('Изменить router')).toBeInTheDocument();
    expect(screen.getByLabelText('Имя')).toBeDisabled();
    await userEvent.click(screen.getByRole('button', { name: 'Отмена' }));
    expect(screen.getAllByText('Новый шаблон').length).toBeGreaterThan(1);
  });

  it('показывает доступный статус бэкапов и ошибку снимка', async () => {
    stubFetch({
      '/api/settings/backups': {
        valkey: {
          lastSave: '2026-11-25T04:30:00Z',
          saveInProgress: true,
          keys: 120,
          memory: '1.2M',
        },
        host: {
          available: true,
          lastRun: '2026-11-25T04:00:00Z',
          result: 'ok',
          size: 2048,
          keepDays: 7,
          restoreTest: true,
        },
      },
      '/api/settings/backups/snapshot': () =>
        new Response(JSON.stringify({ error: { code: 'x', message: 'снимок не запущен' } }), {
          status: 500,
          headers: { 'Content-Type': 'application/json' },
        }),
    });
    renderUI(<BackupsPanel />);
    expect(await screen.findByText('выполняется')).toBeInTheDocument();
    expect(screen.getByText(/проверка восстановления да/)).toBeInTheDocument();
    expect(screen.getByText(/хранить 7 дней/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Снимок сейчас' }));
    expect(await screen.findByText('снимок не запущен')).toBeInTheDocument();
  });

  it('молчит при недоступной системе и показывает пустую историю', async () => {
    stubFetch({
      '/api/settings/system': () => new Response('', { status: 500 }),
      '/api/settings/history': () => new Response('', { status: 500 }),
    });
    renderUI(
      <>
        <SystemPanel />
        <HistoryPanel />
      </>,
    );
    expect(await screen.findByText('Изменений пока нет.')).toBeInTheDocument();
    expect(screen.queryByText('Система')).not.toBeInTheDocument();
  });
});
