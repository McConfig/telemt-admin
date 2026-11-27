import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { Route, Routes } from 'react-router-dom';
import UserPage from './UserPage';
import { me, proxyUser } from '../test/data';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';
import { chooseSelect } from '../test/select';

const riskDay = new Date().toISOString().slice(0, 10);
const dayBefore = new Date(Date.now() - 86_400_000).toISOString().slice(0, 10);

const riskReport = {
  user: 'main',
  level: 1,
  custom: true,
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
      severity: 2,
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
          dayFirst: '2026-11-25T10:00:00Z',
          dayLast: '2026-11-25T11:00:00Z',
          count: 5,
          new: true,
          trusted: false,
        },
        {
          ip: '10.0.0.1',
          internal: true,
          dayFirst: '2026-11-25T10:00:00Z',
          dayLast: '2026-11-25T11:00:00Z',
          count: 3,
          new: false,
          trusted: true,
        },
      ],
    },
    {
      day: dayBefore,
      distinct: 1,
      max: 1,
      newIps: 0,
      newCountries: [],
      traffic: 1024,
      ips: [],
    },
  ],
};

const riskyUser = {
  ...proxyUser,
  risk: 2,
  newCountries24h: 1,
  countries: ['Россия', 'Нидерланды'],
};

const historyUser = {
  ...proxyUser,
  ipHistory: [
    {
      ip: '5.129.63.108',
      firstSeen: '2026-11-20T10:00:00Z',
      lastSeen: '2026-11-25T10:00:00Z',
      count: 30,
      country: 'RU',
      countryName: 'Россия',
      asn: 31200,
      asnOrg: 'Novotelecom Ltd',
    },
    {
      ip: '10.0.0.5',
      firstSeen: '2026-11-19T10:00:00Z',
      lastSeen: '2026-11-24T10:00:00Z',
      count: 2,
      internal: true,
    },
    {
      ip: '203.0.113.9',
      firstSeen: '2026-11-18T10:00:00Z',
      lastSeen: '2026-11-23T10:00:00Z',
      count: 7,
    },
  ],
};

const templates = {
  default: 'router',
  templates: [
    {
      name: 'router',
      maxIps: 5,
      maxTcpConns: 20,
      dataQuotaBytes: 10 * 1024 ** 3,
      quotaPeriod: 'monthly',
      rateLimitUpBps: 1000,
      rateLimitDownBps: 2000,
      expirationDays: 7,
      sourceDeny: ['203.0.113.0/24'],
      riskWarnIps: 4,
      riskHighIps: 6,
      riskDistinct7d: 20,
      riskNewCountries: false,
    },
    { name: 'phone', maxIps: 2 },
  ],
};

function calls(method: string, part: string) {
  return vi
    .mocked(fetch)
    .mock.calls.filter(
      ([url, init]) =>
        String(url).includes(part) && (init as RequestInit | undefined)?.method === method,
    );
}

function baseRoutes() {
  return {
    '/api/meta': { version: 'v1.0.0', repo: 'https://example.com/repo', defaultChartDays: 14 },
    '/api/settings/templates': templates,
    '/api/users/main/traffic': [
      { day: '2026-11-24', bytes: 1024 },
      { day: '2026-11-25', bytes: 2048 },
    ],
    '/api/users/main/timeline': [
      {
        day: '2026-11-25',
        newIps: 1,
        newCountries: [{ code: 'NL', name: 'Нидерланды' }],
        maxIps: 2,
        bytes: 1024,
      },
    ],
    '/api/users/main/trusted': { countries: ['TR'], ips: ['192.0.2.50'] },
    '/api/users/main/note': { note: 'телефон' },
    '/api/users/main/risk-report': riskReport,
    '/api/audit': [],
    '/api/users/main': riskyUser,
  };
}

function renderUser(routes: Record<string, unknown> = {}) {
  stubFetch({ ...baseRoutes(), ...routes });
  return renderUI(
    <Routes>
      <Route path="/users/:name" element={<UserPage me={me} />} />
      <Route path="/" element={<div>список клиентов</div>} />
    </Routes>,
    '/users/main',
  );
}

describe('UserPage — расширенные сценарии', () => {
  it('показывает пустые графики и настройки по умолчанию при сбоях', async () => {
    renderUser({
      '/api/meta': { version: 'v1', repo: 'x', defaultChartDays: 14 },
      '/api/users/main/traffic': () => new Response('', { status: 500 }),
      '/api/users/main/timeline': () => new Response('', { status: 500 }),
      '/api/settings/templates': () => new Response('', { status: 500 }),
    });
    expect(await screen.findByText('14 дней')).toBeInTheDocument();
    expect(await screen.findByText(/Данных пока нет/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Устройства' }));
    expect(await screen.findByText('Событий пока нет')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Заполнить из шаблона' })).not.toBeInTheDocument();
  });

  it('показывает ошибку загрузки клиента и возвращает к списку', async () => {
    renderUser({
      '/api/users/main': () =>
        new Response(JSON.stringify({ error: { code: 'not_found', message: 'клиент не найден' } }), {
          status: 404,
          headers: { 'Content-Type': 'application/json' },
        }),
    });
    expect(await screen.findByText('клиент не найден')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: /К списку/ }));
    expect(await screen.findByText('список клиентов')).toBeInTheDocument();
  });

  it('управляет клиентом из шапки: риск, блокировка, переименование', async () => {
    let failDisable = true;
    const renamed = { ...proxyUser, username: 'renamed' };
    renderUser({
      '/api/users/main': () => riskyUser,
      '/api/users/main/disable': () =>
        failDisable
          ? new Response(JSON.stringify({ error: { code: 'x', message: 'не удалось заблокировать' } }), {
              status: 500,
              headers: { 'Content-Type': 'application/json' },
            })
          : proxyUser,
      '/api/users/main/rename': { username: 'renamed' },
      '/api/users/renamed': renamed,
      '/api/users/renamed/traffic': [],
      '/api/users/renamed/timeline': [],
      '/api/users/renamed/trusted': { countries: [], ips: [] },
      '/api/users/renamed/risk-report': riskReport,
      '/api/users/renamed/note': { note: '' },
    });
    expect(await screen.findByText('main')).toBeInTheDocument();
    expect(screen.getByText('высокий риск')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Отчёт по риску' }));
    expect((await screen.findAllByText('Отчёт по риску')).length).toBeGreaterThan(0);
    expect(screen.getByText('Высокий риск')).toBeInTheDocument();

    failDisable = true;
    await userEvent.click(screen.getByRole('button', { name: 'Заблокировать' }));
    await waitFor(() => expect(calls('POST', '/api/users/main/disable')).toHaveLength(1));

    await userEvent.click(screen.getByRole('button', { name: 'Переименовать' }));
    const dialog = await screen.findByRole('dialog');
    await userEvent.type(within(dialog).getByPlaceholderText('Новое имя'), 'renamed');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Переименовать' }));
    expect(await screen.findByText('renamed')).toBeInTheDocument();
    expect(calls('POST', '/api/users/main/rename')).toHaveLength(1);
  });

  it('показывает ошибку переименования', async () => {
    renderUser({
      '/api/users/main/rename': () =>
        new Response(JSON.stringify({ error: { code: 'conflict', message: 'имя занято' } }), {
          status: 409,
          headers: { 'Content-Type': 'application/json' },
        }),
    });
    await screen.findByText('main');
    await userEvent.click(screen.getByRole('button', { name: 'Переименовать' }));
    const dialog = await screen.findByRole('dialog');
    await userEvent.type(within(dialog).getByPlaceholderText('Новое имя'), 'taken');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Переименовать' }));
    expect(await within(dialog).findByText('имя занято')).toBeInTheDocument();
  });

  it('сохраняет заметку и копирует ссылку', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    });
    renderUser({ '/api/users/main/note': { note: 'новый телефон' } });
    await screen.findByText('main');

    const note = screen.getByPlaceholderText('например: телефон Юли');
    expect(note).toHaveValue('тестовый клиент');
    await userEvent.clear(note);
    await userEvent.type(note, 'новый телефон');
    await userEvent.click(screen.getByRole('button', { name: /Сохранить заметку/ }));
    expect(await screen.findByText('сохранено')).toBeInTheDocument();
    const body = JSON.parse(String((calls('PUT', '/api/users/main/note')[0][1] as RequestInit).body));
    expect(body).toMatchObject({ note: 'новый телефон' });

    await userEvent.click(screen.getByRole('button', { name: /Скопировать/ }));
    expect(writeText).toHaveBeenCalledWith(proxyUser.link);
    expect(await screen.findByText('скопировано')).toBeInTheDocument();
  });

  it('удаляет клиента после подтверждения имени', async () => {
    renderUser({
      '/api/users/main': (_url: URL, init?: RequestInit) =>
        init?.method === 'DELETE' ? {} : riskyUser,
    });
    await screen.findByText('main');
    await userEvent.click(screen.getByRole('button', { name: 'Удалить' }));
    const dialog = await screen.findByRole('dialog');
    await userEvent.type(within(dialog).getByPlaceholderText('Имя'), 'main');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Удалить' }));
    expect(await screen.findByText('список клиентов')).toBeInTheDocument();
    expect(calls('DELETE', '/api/users/main')).toHaveLength(1);
  });

  it('удаляет историю IP из вкладки устройств', async () => {
    renderUser({
      '/api/users/main': historyUser,
      '/api/users/main/history': {},
    });
    await screen.findByText('main');
    await userEvent.click(screen.getByRole('button', { name: 'Устройства' }));
    await userEvent.click(await screen.findByRole('button', { name: 'Удалить историю' }));
    const dialog = await screen.findByRole('dialog');
    await userEvent.type(within(dialog).getByPlaceholderText('Имя'), 'main');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Удалить историю' }));
    await waitFor(() => expect(calls('DELETE', '/api/users/main/history')).toHaveLength(1));
  });

  it('сортирует историю IP и показывает мобильную колонку', async () => {
    renderUser({ '/api/users/main': historyUser });
    await screen.findByText('main');
    await userEvent.click(screen.getByRole('button', { name: 'Устройства' }));
    expect(await screen.findByText('5.129.63.108')).toBeInTheDocument();
    for (const label of ['IP', 'Страна', 'Оператор', 'Впервые', 'Последний раз', 'Наблюдений']) {
      const header = screen.queryByRole('button', { name: new RegExp(`^${label}`) });
      if (header) await userEvent.click(header);
    }
    expect(screen.getAllByText(/↑|↓/).length).toBeGreaterThan(0);
  });

  it('показывает историю IP на мобильном экране', async () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({
      matches: true,
      media: '(max-width: 760px)',
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList);
    renderUser({ '/api/users/main': historyUser });
    await screen.findByText('main');
    await userEvent.click(screen.getByRole('button', { name: 'Устройства' }));
    expect(await screen.findByText('5.129.63.108')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Последний раз/ })).toBeInTheDocument();
  });

  it('заполняет лимиты из шаблона, снимает их и сбрасывает квоту', async () => {
    renderUser();
    await screen.findByText('main');
    await userEvent.click(screen.getByRole('button', { name: 'Лимиты' }));

    await chooseSelect('Шаблон', 'phone');
    await chooseSelect('Шаблон', 'router');
    await userEvent.click(screen.getByRole('button', { name: 'Заполнить из шаблона' }));
    expect(screen.getAllByPlaceholderText('например, 3')[0]).toHaveValue('5');

    for (const label of [
      'снять лимит IP',
      'снять лимит соединений',
      'снять квоту',
      'снять лимиты скорости',
      'без срока',
      'очистить список',
    ]) {
      await userEvent.click(screen.getByLabelText(label));
    }
    await userEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
    await waitFor(() => expect(calls('PATCH', '/api/users/main')).toHaveLength(1));
    const body = JSON.parse(String((calls('PATCH', '/api/users/main')[0][1] as RequestInit).body));
    expect(body).toMatchObject({
      clearMaxUniqueIps: true,
      clearMaxTcpConns: true,
      clearQuota: true,
      clearRates: true,
      clearExpiration: true,
      clearSourceDeny: true,
    });

    await userEvent.click(screen.getByRole('button', { name: 'Сбросить квоту' }));
    await waitFor(() => expect(calls('POST', '/api/users/main/reset-quota')).toHaveLength(1));

    await userEvent.click(screen.getByRole('button', { name: 'Сбросить к общим' }));
    expect(await screen.findByText('сохранено')).toBeInTheDocument();
    const riskBody = JSON.parse(String((calls('PUT', '/api/users/main/risk')[0][1] as RequestInit).body));
    expect(riskBody).toMatchObject({
      warnIps: null,
      highIps: null,
      distinct7d: null,
      newCountries: null,
    });
  });

  it('показывает ошибку сохранения шаблона и закрывает диалог', async () => {
    renderUser({
      '/api/settings/templates': (_url: URL, init?: RequestInit) =>
        init?.method === 'POST'
          ? new Response(JSON.stringify({ error: { code: 'x', message: 'шаблон занят' } }), {
              status: 409,
              headers: { 'Content-Type': 'application/json' },
            })
          : templates,
    });
    await screen.findByText('main');
    await userEvent.click(screen.getByRole('button', { name: 'Лимиты' }));
    await userEvent.click(screen.getByRole('button', { name: 'Сохранить как шаблон' }));
    let dialog = await screen.findByRole('dialog');
    await userEvent.type(within(dialog).getByPlaceholderText('Имя шаблона'), 'taken');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Сохранить' }));
    expect(await within(dialog).findByText('шаблон занят')).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole('button', { name: 'Отмена' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

    await userEvent.click(screen.getByRole('button', { name: 'Сохранить как шаблон' }));
    dialog = await screen.findByRole('dialog');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Отмена' }));
  });

  it('переключает дни риска, период и доверяет страну', async () => {
    renderUser({ '/api/users/main': riskyUser });
    await screen.findByText('main');
    await userEvent.click(screen.getByRole('button', { name: 'Риск' }));
    expect(await screen.findByText('Новые страны за сутки: NL')).toBeInTheDocument();
    expect(screen.getByText(/Пороги свои/)).toBeInTheDocument();
    expect(screen.getByText(/Страны сейчас:/)).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Доверять NL' }));
    await waitFor(() => expect(calls('PUT', '/api/users/main/trusted')).toHaveLength(1));
    const body = JSON.parse(String((calls('PUT', '/api/users/main/trusted')[0][1] as RequestInit).body));
    expect(body.countries).toContain('NL');

    const dayButtons = document.querySelectorAll('.risk-day');
    expect(dayButtons.length).toBe(2);
    await userEvent.click(dayButtons[1] as HTMLElement);
    expect(dayButtons[1]).toHaveAttribute('aria-pressed', 'true');
    expect(await screen.findByText('За этот день адресов нет.')).toBeInTheDocument();

    await chooseSelect('Период', '30 дней');
    await waitFor(() =>
      expect(
        vi.mocked(fetch).mock.calls.some(([url]) => String(url).includes('risk-report?days=30')),
      ).toBe(true),
    );
    expect(calls('GET', '/api/users/main/risk-report').length).toBeGreaterThanOrEqual(2);
  });

  it('показывает успешный риск без причин и без истории', async () => {
    renderUser({
      '/api/users/main/risk-report': {
        ...riskReport,
        custom: false,
        reasons: [],
        current: { ...riskReport.current, countries: [] },
        days: [],
        thresholds: { ...riskReport.thresholds, newCountries: false },
      },
    });
    await screen.findByText('main');
    await userEvent.click(screen.getByRole('button', { name: 'Риск' }));
    expect(await screen.findByText('Риск в пределах действующих порогов.')).toBeInTheDocument();
    expect(screen.getByText('Истории адресов пока нет.')).toBeInTheDocument();
    expect(screen.getByText(/новые страны не учитываются/)).toBeInTheDocument();
  });

  it('показывает отсутствие отчёта риска и ошибку загрузки', async () => {
    renderUser({
      '/api/users/main/risk-report': () =>
        new Response(JSON.stringify({ error: { code: 'x', message: 'отчёт недоступен' } }), {
          status: 500,
          headers: { 'Content-Type': 'application/json' },
        }),
    });
    await screen.findByText('main');
    await userEvent.click(screen.getByRole('button', { name: 'Риск' }));
    expect(await screen.findByText('отчёт недоступен')).toBeInTheDocument();
    expect(screen.getByText('Нет данных')).toBeInTheDocument();
  });

  it('показывает пустой аудит и аудит клиента', async () => {
    const event = {
      at: '2026-11-25T10:00:00Z',
      actor: 'admin',
      action: 'user.rotate',
      target: 'main',
      result: 'ok',
      ip: '192.0.2.1',
    };
    const first = renderUser({ '/api/audit': [] });
    await screen.findByText('main');
    await userEvent.click(screen.getByRole('button', { name: 'Аудит' }));
    expect(await screen.findByText('Событий нет')).toBeInTheDocument();

    first.unmount();
    stubFetch({ ...baseRoutes(), '/api/audit': [event] });
    renderUI(
      <Routes>
        <Route path="/users/:name" element={<UserPage me={me} />} />
      </Routes>,
      '/users/main',
    );
    await screen.findByText('main');
    await userEvent.click(screen.getByRole('button', { name: 'Аудит' }));
    expect(await screen.findByText('user.rotate')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'весь аудит' })).toHaveAttribute(
      'href',
      '/api/audit.csv',
    );
  });

  it('скрывает административные вкладки от viewer', async () => {
    const viewer = { ...me, user: { name: 'viewer', role: 'viewer' as const } };
    stubFetch({ ...baseRoutes(), '/api/users/main': { ...riskyUser, link: undefined, note: 'секретная заметка' } });
    renderUI(
      <Routes>
        <Route path="/users/:name" element={<UserPage me={viewer} />} />
      </Routes>,
      '/users/main',
    );
    expect(await screen.findByText('main')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Лимиты' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Перевыпустить секрет' })).not.toBeInTheDocument();
    expect(screen.queryByText(/секретная заметка/)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Устройства' }));
    expect(screen.getAllByText('Устройства').length).toBeGreaterThan(0);
  });

  it('открывает диалог перевыпуска из вкладки риска', async () => {
    renderUser({ '/api/users/main/rotate': proxyUser });
    await screen.findByText('main');
    await userEvent.click(screen.getByRole('button', { name: 'Риск' }));
    const buttons = await screen.findAllByRole('button', { name: 'Перевыпустить секрет' });
    await userEvent.click(buttons[buttons.length - 1]);
    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Отмена' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });
});
