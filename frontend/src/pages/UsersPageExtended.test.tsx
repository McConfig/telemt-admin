import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import UsersPage from './UsersPage';
import { me, proxyUser, secondUser, summary } from '../test/data';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';
import { chooseSelect } from '../test/select';

function calls(method: string, part: string) {
  return vi
    .mocked(fetch)
    .mock.calls.filter(
      ([url, init]) =>
        String(url).includes(part) && (init as RequestInit | undefined)?.method === method,
    );
}

function manyUsers() {
  return Array.from({ length: 21 }, (_, index) => ({
    ...proxyUser,
    username: `user${String(index + 1).padStart(2, '0')}`,
    risk: 0,
    foreignIpsCurrent: 0,
    newCountries24h: 0,
    countries: [],
  }));
}

describe('UsersPage — расширенные сценарии', () => {
  it('сортирует по всем колонкам и фильтрует по риску и новым странам', async () => {
    stubFetch({ '/api/users': [proxyUser, secondUser], '/api/summary': summary });
    renderUI(<UsersPage me={me} />);
    await screen.findByRole('link', { name: 'main' });

    for (const label of [
      'Имя',
      'Статус',
      'Активные IP',
      'Страны',
      'Новые страны',
      'Иностранные',
      'IP за 24ч',
      'Риск',
      'Соединения',
      'Трафик',
      'Последняя активность',
    ]) {
      await userEvent.click(screen.getByRole('button', { name: new RegExp(`^${label}`) }));
    }
    expect(screen.getAllByText(/↑|↓/).length).toBeGreaterThanOrEqual(11);

    await userEvent.click(screen.getByLabelText('только подозрительные'));
    await waitFor(() => expect(screen.queryByRole('link', { name: 'main' })).not.toBeInTheDocument());
    expect(screen.getByRole('link', { name: 'chernyh' })).toBeInTheDocument();

    await userEvent.click(screen.getByLabelText('только подозрительные'));
    await userEvent.click(screen.getByLabelText('только с новыми странами'));
    await waitFor(() => expect(screen.queryByRole('link', { name: 'main' })).not.toBeInTheDocument());

    await userEvent.click(screen.getByLabelText('только с новыми странами'));
    await userEvent.click(screen.getByLabelText('Выбрать всех'));
    expect(screen.getByText('Выбрано: 2')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Снять выбор' }));
    expect(screen.queryByText(/Выбрано/)).not.toBeInTheDocument();
  });

  it('выполняет массовые включение и отключение, экспортирует ссылки', async () => {
    const createObjectURL = vi.fn().mockReturnValue('blob:links');
    const revokeObjectURL = vi.fn();
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: createObjectURL });
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: revokeObjectURL });

    stubFetch({
      '/api/users': [proxyUser, secondUser],
      '/api/summary': summary,
      '/api/users/main/enable': proxyUser,
      '/api/users/chernyh/enable': secondUser,
      '/api/users/main/disable': proxyUser,
    });
    renderUI(<UsersPage me={me} />);
    await screen.findByRole('link', { name: 'main' });

    await userEvent.click(screen.getByLabelText('Выбрать main'));
    await userEvent.click(screen.getByRole('button', { name: 'Включить' }));
    await waitFor(() => expect(calls('POST', '/api/users/main/enable')).toHaveLength(1));

    await userEvent.click(screen.getByLabelText('Выбрать chernyh'));
    await userEvent.click(screen.getByRole('button', { name: 'Отключить' }));
    await waitFor(() => expect(calls('POST', '/api/users/chernyh/disable')).toHaveLength(1));

    await userEvent.click(screen.getByLabelText('Выбрать main'));
    await userEvent.click(screen.getByRole('button', { name: 'Ссылки CSV' }));
    expect(createObjectURL).toHaveBeenCalled();
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:links');
  });

  it('показывает ошибку массового действия', async () => {
    stubFetch({
      '/api/users': [proxyUser, secondUser],
      '/api/summary': summary,
      '/api/users/main/disable': () =>
        new Response(JSON.stringify({ error: { code: 'x', message: 'нельзя отключить' } }), {
          status: 409,
          headers: { 'Content-Type': 'application/json' },
        }),
    });
    renderUI(<UsersPage me={me} />);
    await screen.findByRole('link', { name: 'main' });
    await userEvent.click(screen.getByLabelText('Выбрать main'));
    await userEvent.click(screen.getByRole('button', { name: 'Отключить' }));
    expect(await screen.findByText('нельзя отключить')).toBeInTheDocument();
  });

  it('создаёт пользователя через диалог и показывает ошибку', async () => {
    let fail = false;
    stubFetch({
      '/api/users': (_url: URL, init?: RequestInit) => {
        if (init?.method !== 'POST') return [proxyUser, secondUser];
        return fail
          ? new Response(JSON.stringify({ error: { code: 'conflict', message: 'уже существует' } }), {
              status: 409,
              headers: { 'Content-Type': 'application/json' },
            })
          : { username: 'created' };
      },
      '/api/summary': summary,
    });
    renderUI(<UsersPage me={me} />);
    await screen.findByRole('link', { name: 'main' });

    await userEvent.click(screen.getByRole('button', { name: 'Создать пользователя' }));
    let dialog = await screen.findByRole('dialog');
    await userEvent.type(within(dialog).getByPlaceholderText('Имя'), 'created');
    await userEvent.type(
      within(dialog).getByPlaceholderText('Секрет (32 hex, пусто — сгенерировать)'),
      'aabb',
    );
    await userEvent.type(within(dialog).getByPlaceholderText('Максимум уникальных IP'), '4');
    await userEvent.type(within(dialog).getByPlaceholderText('Максимум соединений'), '8');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Создать' }));
    await waitFor(() => expect(calls('POST', '/api/users')).toHaveLength(1));
    const body = JSON.parse(String((calls('POST', '/api/users')[0][1] as RequestInit).body));
    expect(body).toMatchObject({
      name: 'created',
      secret: 'aabb',
      maxUniqueIps: 4,
      maxTcpConns: 8,
    });
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

    fail = true;
    await userEvent.click(screen.getByRole('button', { name: 'Создать пользователя' }));
    dialog = await screen.findByRole('dialog');
    await userEvent.type(within(dialog).getByPlaceholderText('Имя'), 'dup');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Создать' }));
    expect(await within(dialog).findByText('уже существует')).toBeInTheDocument();
  });

  it('листает страницы при большом списке', async () => {
    stubFetch({ '/api/users': manyUsers(), '/api/summary': summary });
    renderUI(<UsersPage me={me} />);
    await screen.findByRole('link', { name: 'user01' });
    expect(screen.queryByRole('link', { name: 'user21' })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Вперёд' }));
    expect(await screen.findByRole('link', { name: 'user21' })).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Назад' }));
    expect(await screen.findByRole('link', { name: 'user01' })).toBeInTheDocument();
  });

  it('показывает ошибку загрузки и состояние сборщика', async () => {
    stubFetch({
      '/api/users': () =>
        new Response(JSON.stringify({ error: { code: 'forbidden', message: 'нет доступа к списку' } }), {
          status: 403,
          headers: { 'Content-Type': 'application/json' },
        }),
      '/api/summary': summary,
    });
    const view = renderUI(<UsersPage me={me} />);
    expect(await screen.findByText('нет доступа к списку')).toBeInTheDocument();
    view.unmount();

    stubFetch({
      '/api/users': [proxyUser],
      '/api/summary': {
        ...summary,
        riskHigh: 0,
        riskWarn: 1,
        collector: { up: false },
        geoip: { loaded: false },
      },
    });
    renderUI(<UsersPage me={me} />);
    expect(await screen.findByText(/Сборщик статистики не отвечает/)).toBeInTheDocument();
    expect(screen.getByText(/GeoIP: не загружены/)).toBeInTheDocument();
  });

  it('работает без сводки и на мобильном экране', async () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({
      matches: true,
      media: '(max-width: 760px)',
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList);
    stubFetch({
      '/api/users': [proxyUser, secondUser],
      '/api/summary': () => new Response('', { status: 500 }),
    });
    renderUI(<UsersPage me={me} />);
    expect(await screen.findAllByText(/^IP:/)).toHaveLength(2);
    expect(screen.getByText(/Россия/)).toBeInTheDocument();
    expect(screen.getAllByText(/Новых стран за сутки:/)).toHaveLength(1);
    expect(screen.getByText('высокий риск')).toBeInTheDocument();
    await userEvent.click(screen.getAllByLabelText('Выбрать main')[0]);
    expect(screen.getByText('Выбрано: 1')).toBeInTheDocument();
    await chooseSelect('Сортировка', 'по трафику');
    expect(screen.getByRole('combobox', { name: 'Сортировка' })).toHaveTextContent('по трафику');
    await chooseSelect('Сортировка', 'без сортировки');
  });

  it('скрывает создание для viewer', async () => {
    stubFetch({ '/api/users': [proxyUser], '/api/summary': summary });
    renderUI(<UsersPage me={{ ...me, user: { name: 'viewer', role: 'viewer' } }} />);
    await screen.findByRole('link', { name: 'main' });
    expect(screen.queryByRole('button', { name: 'Создать пользователя' })).not.toBeInTheDocument();
  });
});

describe('UsersPage — импорт CSV', () => {
  const preview = {
    dryRun: true,
    total: 3,
    create: 1,
    skip: 1,
    error: 1,
    rows: [
      { line: 2, name: 'newuser', status: 'create' },
      { line: 3, name: 'main', status: 'skip', message: 'клиент уже существует' },
      { line: 4, name: 'bad name', status: 'error', message: 'имя: недопустимое' },
    ],
  };
  const applied = {
    ...preview,
    dryRun: false,
  };

  function importRoutes() {
    stubFetch({
      '/api/users': [proxyUser],
      '/api/summary': summary,
      '/api/users/import': (url: URL) =>
        url.searchParams.get('dryRun') === '0' ? applied : preview,
    });
  }

  it('проверяет файл и создаёт валидных клиентов', async () => {
    const createObjectURL = vi.fn().mockReturnValue('blob:template');
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: createObjectURL });
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: vi.fn() });
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
    importRoutes();
    renderUI(<UsersPage me={me} />);
    await screen.findByRole('link', { name: 'main' });

    await userEvent.click(screen.getByRole('button', { name: 'Импорт CSV' }));
    expect(await screen.findByText('Импорт клиентов')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Скачать шаблон' }));
    expect(createObjectURL).toHaveBeenCalled();

    const input = screen.getByLabelText('CSV-файл');
    await userEvent.upload(
      input,
      new File(['name\nnewuser\nmain\n'], 'clients.csv', { type: 'text/csv' }),
    );
    expect(await screen.findByText(/clients\.csv/)).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Проверить' }));
    expect(await screen.findByText(/будет создано: 1/)).toBeInTheDocument();
    expect(screen.getByText('клиент уже существует')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Создать 1' }));
    expect(await screen.findByText(/Создано: 1/)).toBeInTheDocument();

    const calls = vi
      .mocked(fetch)
      .mock.calls.filter(([url]) => String(url).includes('/api/users/import'));
    expect(calls).toHaveLength(2);
    expect(String(calls[0][0])).toContain('dryRun=1');
    expect(String(calls[1][0])).toContain('dryRun=0');
    expect(calls[1][1]?.body).toBe('name\nnewuser\nmain\n');
  });

  it('показывает ошибку разбора CSV', async () => {
    stubFetch({
      '/api/users': [proxyUser],
      '/api/summary': summary,
      '/api/users/import': () =>
        new Response(JSON.stringify({ error: { code: 'bad_request', message: 'нет колонки name' } }), {
          status: 400,
          headers: { 'Content-Type': 'application/json' },
        }),
    });
    renderUI(<UsersPage me={me} />);
    await screen.findByRole('link', { name: 'main' });
    await userEvent.click(screen.getByRole('button', { name: 'Импорт CSV' }));
    await userEvent.upload(
      screen.getByLabelText('CSV-файл'),
      new File(['secret\nx\n'], 'bad.csv', { type: 'text/csv' }),
    );
    await userEvent.click(screen.getByRole('button', { name: 'Проверить' }));
    expect(await screen.findByText('нет колонки name')).toBeInTheDocument();
  });
});
