import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import AdminsPage from './AdminsPage';
import { me } from '../test/data';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';

const admin = (overrides: Record<string, unknown> = {}) => ({
  name: 'other',
  role: 'admin',
  disabled: false,
  createdAt: '2026-11-01T00:00:00Z',
  passwordChangedAt: '2026-11-01T00:00:00Z',
  sessions: 2,
  totpEnabled: true,
  ...overrides,
});

const admins = [
  admin(),
  admin({
    name: 'watcher',
    role: 'viewer',
    disabled: true,
    createdAt: '2026-11-02T00:00:00Z',
    passwordChangedAt: '2026-11-02T00:00:00Z',
    sessions: 0,
    totpEnabled: false,
  }),
];

function calls(method: string, part: string) {
  return vi
    .mocked(fetch)
    .mock.calls.filter(
      ([url, init]) =>
        String(url).includes(part) && (init as RequestInit | undefined)?.method === method,
    );
}

describe('AdminsPage', () => {
  it('сортирует по всем колонкам', async () => {
    stubFetch({ '/api/admins': admins });
    renderUI(<AdminsPage me={me} />);
    await screen.findByText('watcher');

    for (const label of ['Имя', 'Роль', 'Статус', '2FA', 'Сессии', 'Создан']) {
      const header = screen.getAllByRole('button', { name: new RegExp(`^${label}`) })[0];
      await userEvent.click(header);
    }
    expect(screen.getAllByText(/↑|↓/).length).toBeGreaterThanOrEqual(6);
  });

  it('включает и отключает пользователя, сбрасывает 2FA, сессии и удаляет', async () => {
    const fetchMock = stubFetch({
      '/api/admins': admins,
      '/api/admins/other/totp-reset': {},
      '/api/admins/other/sessions/revoke': {},
    });
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true);
    renderUI(<AdminsPage me={me} />);
    await screen.findByText('other');

    await userEvent.click(screen.getAllByRole('button', { name: 'Отключить' })[0]);
    await waitFor(() => expect(calls('PATCH', '/api/admins/other')).toHaveLength(1));
    expect(JSON.parse(String((calls('PATCH', '/api/admins/other')[0][1] as RequestInit).body))).toMatchObject({
      disabled: true,
    });

    await userEvent.click(screen.getAllByRole('button', { name: 'Сбросить 2FA' })[0]);
    expect(confirm).toHaveBeenCalledWith('Сбросить 2FA для other?');
    await waitFor(() => expect(calls('POST', '/api/admins/other/totp-reset')).toHaveLength(1));

    await userEvent.click(screen.getAllByRole('button', { name: 'Завершить сессии' })[0]);
    await waitFor(() => expect(calls('POST', '/api/admins/other/sessions/revoke')).toHaveLength(1));

    await userEvent.click(screen.getAllByRole('button', { name: 'Удалить' })[0]);
    await waitFor(() => expect(calls('DELETE', '/api/admins/other')).toHaveLength(1));

    confirm.mockReturnValue(false);
    await userEvent.click(screen.getAllByRole('button', { name: 'Сбросить 2FA' })[0]);
    expect(fetchMock).toHaveBeenCalled();
  });

  it('включает ранее отключённого пользователя', async () => {
    stubFetch({ '/api/admins': [admin({ name: 'watcher', disabled: true, totpEnabled: false })] });
    renderUI(<AdminsPage me={me} />);
    await screen.findByText('watcher');
    await userEvent.click(screen.getByRole('button', { name: 'Включить' }));
    await waitFor(() => expect(calls('PATCH', '/api/admins/watcher')).toHaveLength(1));
    expect(JSON.parse(String((calls('PATCH', '/api/admins/watcher')[0][1] as RequestInit).body))).toMatchObject({
      disabled: false,
    });
  });

  it('создаёт администратора и показывает сгенерированный пароль', async () => {
    stubFetch({
      '/api/admins': (_url: URL, init?: RequestInit) =>
        init?.method === 'POST'
          ? { user: admin({ name: 'newbie' }), password: 'generated-pass' }
          : admins,
    });
    renderUI(<AdminsPage me={me} />);
    await screen.findByText('other');

    await userEvent.click(screen.getByRole('button', { name: 'Создать' }));
    const dialog = await screen.findByRole('dialog');
    await userEvent.type(within(dialog).getByPlaceholderText('Имя'), 'newbie');
    await userEvent.click(within(dialog).getByRole('combobox', { name: 'Роль' }));
    await userEvent.keyboard('{ArrowDown}{Enter}');
    await userEvent.type(
      within(dialog).getByPlaceholderText('Пароль (пусто — сгенерировать)'),
      'secret-pass',
    );
    await userEvent.click(within(dialog).getByRole('button', { name: 'Создать' }));

    expect(await screen.findByText(/generated-pass/)).toBeInTheDocument();
    const body = JSON.parse(String((calls('POST', '/api/admins')[0][1] as RequestInit).body));
    expect(body).toMatchObject({ name: 'newbie', role: 'admin', password: 'secret-pass' });
  });

  it('показывает ошибку создания администратора', async () => {
    stubFetch({
      '/api/admins': (_url: URL, init?: RequestInit) =>
        init?.method === 'POST'
          ? new Response(JSON.stringify({ error: { code: 'conflict', message: 'имя занято' } }), {
              status: 409,
              headers: { 'Content-Type': 'application/json' },
            })
          : admins,
    });
    renderUI(<AdminsPage me={me} />);
    await screen.findByText('other');
    await userEvent.click(screen.getByRole('button', { name: 'Создать' }));
    const dialog = await screen.findByRole('dialog');
    await userEvent.type(within(dialog).getByPlaceholderText('Имя'), 'dup');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Создать' }));
    expect(await within(dialog).findByText('имя занято')).toBeInTheDocument();
  });

  it('меняет пароль пользователя и показывает результат', async () => {
    stubFetch({
      '/api/admins': admins,
      '/api/admins/other/password': { password: 'new-password' },
    });
    renderUI(<AdminsPage me={me} />);
    await screen.findByText('other');
    await userEvent.click(screen.getAllByRole('button', { name: 'Сменить пароль' })[0]);
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('Новый пароль для other')).toBeInTheDocument();
    await userEvent.type(within(dialog).getByPlaceholderText('Пусто — сгенерировать'), 'manual');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Сменить' }));
    expect(await screen.findByText(/new-password/)).toBeInTheDocument();
    const body = JSON.parse(String((calls('POST', '/api/admins/other/password')[0][1] as RequestInit).body));
    expect(body).toMatchObject({ password: 'manual' });
  });

  it('показывает ошибку смены пароля', async () => {
    stubFetch({
      '/api/admins': admins,
      '/api/admins/other/password': new Response(
        JSON.stringify({ error: { code: 'bad_request', message: 'пароль слишком короткий' } }),
        { status: 400, headers: { 'Content-Type': 'application/json' } },
      ),
    });
    renderUI(<AdminsPage me={me} />);
    await screen.findByText('other');
    await userEvent.click(screen.getAllByRole('button', { name: 'Сменить пароль' })[0]);
    const dialog = await screen.findByRole('dialog');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Сменить' }));
    expect(await within(dialog).findByText('пароль слишком короткий')).toBeInTheDocument();
  });

  it('показывает ошибку загрузки и мобильные карточки', async () => {
    stubFetch({
      '/api/admins': () =>
        new Response(JSON.stringify({ error: { code: 'forbidden', message: 'нет доступа' } }), {
          status: 403,
          headers: { 'Content-Type': 'application/json' },
        }),
    });
    const view = renderUI(<AdminsPage me={me} />);
    expect(await screen.findByText('нет доступа')).toBeInTheDocument();
    view.unmount();

    vi.spyOn(window, 'confirm').mockReturnValue(true);
    vi.spyOn(window, 'matchMedia').mockReturnValue({
      matches: true,
      media: '(max-width: 760px)',
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList);
    stubFetch({
      '/api/admins': admins,
      '/api/admins/other/totp-reset': {},
    });
    renderUI(<AdminsPage me={me} />);
    expect(await screen.findByText(/сессий: 2/)).toBeInTheDocument();
    await userEvent.click(screen.getAllByRole('button', { name: 'Сбросить 2FA' })[0]);
  });

  it('закрывает раздел для viewer', () => {
    stubFetch({});
    renderUI(<AdminsPage me={{ ...me, user: { name: 'viewer', role: 'viewer' } }} />);
    expect(screen.getByText('Нет прав')).toBeInTheDocument();
  });
});
