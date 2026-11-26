import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import AccountPage from './AccountPage';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';

const sessions = [
  {
    id: 'a',
    userAgent: 'Safari',
    ip: '192.0.2.2',
    createdAt: '2026-11-01T00:00:00Z',
    current: false,
  },
  {
    id: 'b',
    userAgent: '',
    ip: '192.0.2.1',
    createdAt: '2026-11-02T00:00:00Z',
    current: true,
  },
];

const setup = {
  secret: 'JBSWY3DPEHPK3PXP',
  url: 'otpauth://totp/x',
  qr: 'data:image/png;base64,AAAA',
};

function calls(method: string, part: string) {
  return vi
    .mocked(fetch)
    .mock.calls.filter(
      ([url, init]) =>
        String(url).includes(part) && (init as RequestInit | undefined)?.method === method,
    );
}

describe('AccountPage', () => {
  it('показывает недоступную 2FA и ошибку сессий', async () => {
    stubFetch({
      '/api/account/totp': { available: false, enabled: false, recoveryLeft: 0 },
      '/api/account/sessions': () =>
        new Response(JSON.stringify({ error: { code: 'db', message: 'база недоступна' } }), {
          status: 500,
          headers: { 'Content-Type': 'application/json' },
        }),
    });
    renderUI(<AccountPage />);
    expect(await screen.findByText('база недоступна')).toBeInTheDocument();
    expect(screen.getByText(/Недоступна: на сервере не задан ключ/)).toBeInTheDocument();
    expect(screen.getByText('Других сессий нет')).toBeInTheDocument();
  });

  it('показывает неизвестное состояние 2FA и ошибку смены пароля', async () => {
    stubFetch({
      '/api/account/totp': () => new Response('', { status: 500 }),
      '/api/account/sessions': [],
      '/api/account/password': () =>
        new Response(JSON.stringify({ error: { code: 'bad', message: 'неверный пароль' } }), {
          status: 400,
          headers: { 'Content-Type': 'application/json' },
        }),
    });
    renderUI(<AccountPage />);
    expect(await screen.findByText('Не удалось узнать состояние 2FA')).toBeInTheDocument();
    await userEvent.type(screen.getByPlaceholderText('Текущий пароль'), 'old password 123');
    await userEvent.type(
      screen.getByPlaceholderText('Новый пароль (не короче 12)'),
      'new password 1234',
    );
    await userEvent.click(screen.getByRole('button', { name: 'Сменить' }));
    expect(await screen.findByText('неверный пароль')).toBeInTheDocument();
  });

  it('сортирует сессии по устройству, IP и дате', async () => {
    stubFetch({
      '/api/account/sessions': sessions,
      '/api/account/totp': { available: true, enabled: false, recoveryLeft: 0 },
    });
    renderUI(<AccountPage />);
    await screen.findByText('Safari');
    for (const label of ['Устройство', 'IP', 'Вход']) {
      await userEvent.click(screen.getByRole('button', { name: new RegExp(`^${label}`) }));
    }
    expect(screen.getAllByText(/↑|↓/).length).toBeGreaterThanOrEqual(3);
    expect(screen.getByText('—')).toBeInTheDocument();
    expect(screen.getByText('текущая')).toBeInTheDocument();
  });

  it('выключает 2FA, выпускает новые коды и работает с буфером обмена', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    });
    const createObjectURL = vi.fn().mockReturnValue('blob:codes');
    const revokeObjectURL = vi.fn();
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: createObjectURL });
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: revokeObjectURL });

    stubFetch({
      '/api/account/sessions': [],
      '/api/account/totp': {
        available: true,
        enabled: true,
        createdAt: '2026-11-01T00:00:00Z',
        recoveryLeft: 2,
      },
      '/api/account/totp/disable': {},
      '/api/account/totp/recovery': { recoveryCodes: ['AAAA-BBBB', 'CCCC-DDDD'] },
    });
    renderUI(<AccountPage />);

    expect(await screen.findByText(/Включена с/)).toBeInTheDocument();
    await userEvent.type(screen.getByPlaceholderText('Пароль'), 'secret');
    await userEvent.type(screen.getByPlaceholderText('Код'), '123456');
    await userEvent.click(screen.getByRole('button', { name: 'Выключить' }));
    expect(await screen.findByText('2FA выключена')).toBeInTheDocument();
    expect(calls('POST', '/api/account/totp/disable')).toHaveLength(1);

    await userEvent.type(screen.getByPlaceholderText('Пароль'), 'secret');
    await userEvent.type(screen.getByPlaceholderText('Код'), '654321');
    await userEvent.click(screen.getByRole('button', { name: 'Новые резервные коды' }));
    expect(await screen.findByText(/AAAA-BBBB/)).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Скопировать' }));
    expect(writeText).toHaveBeenCalledWith('AAAA-BBBB\nCCCC-DDDD');
    await userEvent.click(screen.getByRole('button', { name: 'Скачать' }));
    expect(createObjectURL).toHaveBeenCalled();
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:codes');
    await userEvent.click(screen.getByRole('button', { name: 'Скрыть' }));
    expect(screen.queryByText(/AAAA-BBBB/)).not.toBeInTheDocument();
  });

  it('показывает ошибки выключения и перевыпуска кодов', async () => {
    stubFetch({
      '/api/account/sessions': [],
      '/api/account/totp': { available: true, enabled: true, recoveryLeft: 2 },
      '/api/account/totp/disable': () =>
        new Response(JSON.stringify({ error: { code: 'bad', message: 'неверный код' } }), {
          status: 400,
          headers: { 'Content-Type': 'application/json' },
        }),
      '/api/account/totp/recovery': () =>
        new Response(JSON.stringify({ error: { code: 'bad', message: 'нет кодов' } }), {
          status: 400,
          headers: { 'Content-Type': 'application/json' },
        }),
    });
    renderUI(<AccountPage />);
    await screen.findByText(/Включена с/);
    await userEvent.click(screen.getByRole('button', { name: 'Выключить' }));
    expect(await screen.findByText('неверный код')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Новые резервные коды' }));
    expect(await screen.findByText('нет кодов')).toBeInTheDocument();
  });

  it('включает 2FA, вызывает onMeChange и показывает ошибки настройки', async () => {
    let failSetup = false;
    let failEnable = false;
    stubFetch({
      '/api/account/sessions': [],
      '/api/account/totp': { available: true, enabled: false, recoveryLeft: 0 },
      '/api/account/totp/setup': () =>
        failSetup
          ? new Response(JSON.stringify({ error: { code: 'x', message: 'ключ не задан' } }), {
              status: 500,
              headers: { 'Content-Type': 'application/json' },
            })
          : setup,
      '/api/account/totp/enable': () =>
        failEnable
          ? new Response(JSON.stringify({ error: { code: 'x', message: 'код неверный' } }), {
              status: 400,
              headers: { 'Content-Type': 'application/json' },
            })
          : { recoveryCodes: ['ONE-TWO'] },
    });
    const onMeChange = vi.fn();
    renderUI(<AccountPage onMeChange={onMeChange} />);
    await screen.findByRole('button', { name: 'Включить 2FA' });

    failSetup = true;
    await userEvent.click(screen.getByRole('button', { name: 'Включить 2FA' }));
    expect(await screen.findByText('ключ не задан')).toBeInTheDocument();

    failSetup = false;
    await userEvent.click(screen.getByRole('button', { name: 'Включить 2FA' }));
    expect(await screen.findByText('JBSWY3DPEHPK3PXP')).toBeInTheDocument();
    expect(screen.getByAltText('QR для аутентификатора')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Отмена' }));
    expect(screen.queryByText('JBSWY3DPEHPK3PXP')).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Включить 2FA' }));
    const code = await screen.findByPlaceholderText('Код из приложения');
    failEnable = true;
    await userEvent.type(code, '123456');
    await userEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));
    expect(await screen.findByText('код неверный')).toBeInTheDocument();

    failEnable = false;
    await userEvent.clear(code);
    await userEvent.type(code, '654321');
    await userEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));
    expect(await screen.findByText('ONE-TWO')).toBeInTheDocument();
    expect(onMeChange).toHaveBeenCalled();
  });

  it('показывает мобильные карточки сессий и завершает сессию', async () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({
      matches: true,
      media: '(max-width: 760px)',
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList);
    stubFetch({
      '/api/account/sessions': sessions,
      '/api/account/totp': { available: true, enabled: false, recoveryLeft: 0 },
      '/api/account/sessions/a/revoke': {},
    });
    renderUI(<AccountPage />);
    expect(await screen.findByText('устройство не определено')).toBeInTheDocument();
    expect(screen.getByText('текущая')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Завершить' }));
    await waitFor(() => expect(calls('POST', '/api/account/sessions/a/revoke')).toHaveLength(1));
  });

  it('закрывает ошибки 2FA и сессий по крестику', async () => {
    stubFetch({
      '/api/account/sessions': () =>
        new Response(JSON.stringify({ error: { code: 'x', message: 'ошибка сессий' } }), {
          status: 500,
          headers: { 'Content-Type': 'application/json' },
        }),
      '/api/account/totp': { available: true, enabled: false, recoveryLeft: 0 },
    });
    renderUI(<AccountPage />);
    const alert = await screen.findByText('ошибка сессий');
    expect(alert).toBeInTheDocument();
    const close = within(alert.closest('.alert') ?? document.body).getAllByRole('button')[0];
    if (close) await userEvent.click(close);
    expect(screen.queryByText('ошибка сессий')).not.toBeInTheDocument();
  });
});
