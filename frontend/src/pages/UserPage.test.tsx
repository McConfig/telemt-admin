import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { Route, Routes } from 'react-router-dom';
import UserPage from './UserPage';
import { me, proxyUser } from '../test/data';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';

const riskDay = new Date().toISOString().slice(0, 10);
const riskReport = {
  user: 'main',
  level: 1,
  custom: false,
  thresholds: { warnIps: 3, highIps: 5, distinct7d: 15, newCountries: true },
  current: {
    activeIps: 2,
    trustedActive: 0,
    current: 2,
    distinct24h: 2,
    distinct7d: 4,
    maxToday: 2,
    newIps24h: 1,
    newCountries: 1,
    foreignIps24h: 1,
    countries: [{ code: 'NL', name: 'Нидерланды' }],
  },
  reasons: [
    {
      code: 'new_country',
      severity: 1,
      value: 1,
      countries: ['NL'],
      message: 'Новые страны за сутки: NL',
    },
  ],
  days: [
    {
      day: riskDay,
      distinct: 2,
      max: 2,
      newIps: 1,
      newCountries: [{ code: 'NL', name: 'Нидерланды' }],
      traffic: 2048,
      ips: [
        {
          ip: '198.51.100.7',
          country: 'NL',
          countryName: 'Нидерланды',
          asn: 60781,
          asnOrg: 'LeaseWeb',
          dayFirst: new Date().toISOString(),
          dayLast: new Date().toISOString(),
          count: 5,
          new: true,
          trusted: false,
        },
        {
          ip: '192.0.2.10',
          country: 'RU',
          countryName: 'Россия',
          dayFirst: new Date().toISOString(),
          dayLast: new Date().toISOString(),
          count: 40,
          new: false,
          trusted: false,
        },
      ],
    },
  ],
};

function renderUser() {
  stubFetch({
    '/api/users/main/traffic': [{ day: '2026-11-24', bytes: 1024 }],
    '/api/users/main/timeline': [
      { day: '2026-11-24', newIps: 1, maxIps: 2, bytes: 1024 },
    ],
    '/api/users/main/trusted': { countries: [], ips: [] },
    '/api/users/main/note': { note: 'телефон' },
    '/api/users/main/risk-report': riskReport,
    '/api/users/main/quota-reset': {
      mode: 'inherit',
      enabled: true,
      useDays: 30,
      useTime: '04:00',
    },
    '/api/settings/templates': {
      default: 'router',
      templates: [{ name: 'router', maxIps: 5 }],
    },
    '/api/users/main/rotate': proxyUser,
    '/api/users/main/disable': proxyUser,
    '/api/users/main': proxyUser,
    '/api/audit': [
      {
        at: new Date().toISOString(),
        actor: 'admin',
        action: 'user.rotate',
        target: 'main',
        result: 'ok',
        ip: '192.0.2.1',
      },
    ],
  });
  renderUI(
    <Routes>
      <Route path="/users/:name" element={<UserPage me={me} />} />
    </Routes>,
    '/users/main',
  );
}

describe('UserPage', () => {
  it('переключает вкладки и сохраняет доверенные страны', async () => {
    renderUser();
    expect(await screen.findByText('main')).toBeInTheDocument();
    expect(screen.getByText('Трафик по дням')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Устройства' }));
    expect(screen.getByText('Активность по дням')).toBeInTheDocument();
    await userEvent.type(screen.getByPlaceholderText('например, TR, GE'), 'TR');
    await userEvent.click(screen.getByRole('button', { name: 'Сохранить доверенные' }));
    expect(await screen.findByText('сохранено')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Лимиты' }));
    expect(screen.getByText('Квота трафика')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Аудит' }));
    expect(await screen.findByText('Аудит клиента')).toBeInTheDocument();
  });

  it('показывает отчёт по риску и доверяет адрес', async () => {
    renderUser();
    const mock = vi.mocked(fetch);
    await screen.findByText('main');
    await userEvent.click(screen.getByRole('button', { name: 'Риск' }));

    expect(await screen.findByText('Новые страны за сутки: NL')).toBeInTheDocument();
    expect(screen.getByText('198.51.100.7')).toBeInTheDocument();
    expect(screen.getByText('LeaseWeb')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Экспорт CSV' })).toHaveAttribute(
      'href',
      expect.stringContaining('/risk.csv'),
    );

    await userEvent.click(screen.getAllByRole('button', { name: 'Доверять IP' })[0]);
    await waitFor(() => {
      const call = mock.mock.calls.find(
        ([url, init]) =>
          String(url).includes('/trusted') && (init as RequestInit | undefined)?.method === 'PUT',
      );
      expect(call).toBeTruthy();
      expect(JSON.parse(String((call?.[1] as RequestInit).body))).toMatchObject({
        ips: ['198.51.100.7'],
      });
    });
  });

  it('сохраняет пороги риска', async () => {
    renderUser();
    const mock = vi.mocked(fetch);
    await screen.findByText('main');
    await userEvent.click(screen.getByRole('button', { name: 'Лимиты' }));

    await userEvent.type(screen.getByLabelText('Предупреждение, больше IP'), '6');
    await userEvent.type(screen.getByLabelText('Высокий риск, больше IP'), '8');
    await userEvent.selectOptions(screen.getByLabelText('Реакция на новую страну'), 'no');
    await userEvent.click(screen.getByRole('button', { name: 'Сохранить пороги' }));

    expect(await screen.findByText('сохранено')).toBeInTheDocument();
    const call = mock.mock.calls.find(([url]) => String(url).includes('/risk'));
    expect(call).toBeTruthy();
    const init = call?.[1] as RequestInit;
    expect(JSON.parse(String(init.body))).toMatchObject({
      warnIps: 6,
      highIps: 8,
      distinct7d: null,
      newCountries: false,
    });
  });

  it('настраивает автосброс квоты', async () => {
    renderUser();
    await screen.findByText('main');
    await userEvent.click(screen.getByRole('button', { name: 'Лимиты' }));
    expect(await screen.findByText('Автосброс квоты')).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText('Режим'), 'custom');
    await userEvent.type(screen.getByLabelText('Интервал, дней'), '14');
    await userEvent.type(screen.getByLabelText('Время'), '05:30');
    await userEvent.click(screen.getByRole('button', { name: 'Сохранить расписание' }));
    const call = vi
      .mocked(fetch)
      .mock.calls.find(
        ([url, init]) =>
          String(url).endsWith('/quota-reset') &&
          (init as RequestInit | undefined)?.method === 'PUT',
      );
    expect(call).toBeTruthy();
    expect(JSON.parse(String((call?.[1] as RequestInit).body))).toMatchObject({
      mode: 'custom',
      days: 14,
      time: '05:30',
    });
  });

  it('сохраняет текущие лимиты как шаблон', async () => {
    renderUser();
    await screen.findByText('main');
    await userEvent.click(screen.getByRole('button', { name: 'Лимиты' }));
    await userEvent.click(screen.getByRole('button', { name: 'Сохранить как шаблон' }));
    const dialog = await screen.findByRole('dialog');
    await userEvent.type(screen.getByPlaceholderText('Имя шаблона'), 'phone');
    const apply = Array.from(dialog.querySelectorAll('button')).find(
      (b) => b.textContent === 'Сохранить',
    );
    if (apply) await userEvent.click(apply);
    const mock = vi.mocked(fetch);
    await waitFor(() => {
      const call = mock.mock.calls.find(
        ([url, init]) =>
          String(url).endsWith('/api/settings/templates') &&
          (init as RequestInit | undefined)?.method === 'POST',
      );
      expect(call).toBeTruthy();
      expect(JSON.parse(String((call?.[1] as RequestInit).body))).toMatchObject({
        name: 'phone',
      });
    });
  });

  it('перевыпускает секрет через диалог', async () => {
    renderUser();
    await screen.findByText('main');
    await userEvent.click(screen.getByRole('button', { name: 'Перевыпустить секрет' }));
    const dialog = await screen.findByRole('dialog');
    await userEvent.type(screen.getByPlaceholderText('Имя'), 'main');
    const apply = Array.from(dialog.querySelectorAll('button')).find(
      (b) => b.textContent === 'Перевыпустить',
    );
    if (apply) await userEvent.click(apply);
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });
});
