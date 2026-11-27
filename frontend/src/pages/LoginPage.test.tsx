import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { startAuthentication } from '@simplewebauthn/browser';

vi.mock('@simplewebauthn/browser', () => ({ startAuthentication: vi.fn() }));
import LoginPage from './LoginPage';
import { me } from '../test/data';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';

describe('LoginPage', () => {
  it('входит по паролю', async () => {
    stubFetch({ '/api/login': me });
    const onLogin = vi.fn();
    renderUI(<LoginPage onLogin={onLogin} />);
    await userEvent.type(screen.getByPlaceholderText('Имя'), 'admin');
    await userEvent.type(screen.getByPlaceholderText('Пароль'), 'secret');
    await userEvent.click(screen.getByRole('button', { name: 'Войти' }));
    expect(onLogin).toHaveBeenCalledWith(me);
  });

  it('пускает по passkey после пароля', async () => {
    vi.mocked(startAuthentication).mockResolvedValue({ id: 'cred' } as never);
    const onLogin = vi.fn();
    stubFetch({
      '/api/login': { mfaRequired: true, passkey: true, totp: true, totpRequired: true },
      '/api/login/webauthn/begin': {},
      '/api/login/webauthn/finish': me,
    });
    renderUI(<LoginPage onLogin={onLogin} />);
    await userEvent.type(screen.getByPlaceholderText('Имя'), 'admin');
    await userEvent.type(screen.getByPlaceholderText('Пароль'), 'secret');
    await userEvent.click(screen.getByRole('button', { name: 'Войти' }));
    await userEvent.click(await screen.findByRole('button', { name: 'Войти с passkey' }));
    await waitFor(() => expect(onLogin).toHaveBeenCalledWith(me));
  });

  it('показывает второй фактор и принимает код', async () => {
    stubFetch({
      '/api/login': { totpRequired: true },
      '/api/login/totp': me,
    });
    const onLogin = vi.fn();
    renderUI(<LoginPage onLogin={onLogin} />);
    await userEvent.type(screen.getByPlaceholderText('Имя'), 'admin');
    await userEvent.type(screen.getByPlaceholderText('Пароль'), 'secret');
    await userEvent.click(screen.getByRole('button', { name: 'Войти' }));

    expect(await screen.findByText('Второй фактор')).toBeInTheDocument();
    await userEvent.type(screen.getByPlaceholderText('000000'), '123456');
    await userEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));
    expect(onLogin).toHaveBeenCalledWith(me);
  });

  it('переключает резервный код и возвращается назад', async () => {
    stubFetch({ '/api/login': { totpRequired: true } });
    renderUI(<LoginPage onLogin={vi.fn()} />);
    await userEvent.type(screen.getByPlaceholderText('Имя'), 'admin');
    await userEvent.type(screen.getByPlaceholderText('Пароль'), 'secret');
    await userEvent.click(screen.getByRole('button', { name: 'Войти' }));
    await screen.findByText('Второй фактор');
    await userEvent.click(screen.getByRole('button', { name: 'Использовать резервный код' }));
    expect(screen.getByPlaceholderText('XXXX-XXXX')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Назад' }));
    expect(screen.getByText('Вход в TeleMT Admin')).toBeInTheDocument();
  });
});

describe('LoginPage — ошибки', () => {
  it('показывает ошибку входа по паролю', async () => {
    stubFetch({
      '/api/login': () =>
        new Response(JSON.stringify({ error: { code: 'auth', message: 'неверный пароль' } }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        }),
    });
    renderUI(<LoginPage onLogin={vi.fn()} />);
    await userEvent.type(screen.getByPlaceholderText('Имя'), 'admin');
    await userEvent.type(screen.getByPlaceholderText('Пароль'), 'wrong');
    await userEvent.click(screen.getByRole('button', { name: 'Войти' }));
    expect(await screen.findByText('неверный пароль')).toBeInTheDocument();
  });

  it('показывает ошибку проверки второго фактора', async () => {
    stubFetch({
      '/api/login': { totpRequired: true },
      '/api/login/totp': () =>
        new Response(JSON.stringify({ error: { code: 'auth', message: 'код неверный' } }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        }),
    });
    renderUI(<LoginPage onLogin={vi.fn()} />);
    await userEvent.type(screen.getByPlaceholderText('Имя'), 'admin');
    await userEvent.type(screen.getByPlaceholderText('Пароль'), 'secret');
    await userEvent.click(screen.getByRole('button', { name: 'Войти' }));
    await screen.findByText('Второй фактор');
    await userEvent.type(screen.getByPlaceholderText('000000'), '123456');
    await userEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));
    expect(await screen.findByText('код неверный')).toBeInTheDocument();
  });

  it('показывает резервный код и возвращает к форме пароля', async () => {
    stubFetch({
      '/api/login': { totpRequired: true },
      '/api/login/totp': () =>
        new Response(JSON.stringify({ error: { code: 'auth', message: 'резервный код неверный' } }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        }),
    });
    renderUI(<LoginPage onLogin={vi.fn()} />);
    await userEvent.type(screen.getByPlaceholderText('Имя'), 'admin');
    await userEvent.type(screen.getByPlaceholderText('Пароль'), 'secret');
    await userEvent.click(screen.getByRole('button', { name: 'Войти' }));
    await screen.findByText('Второй фактор');
    await userEvent.click(screen.getByRole('button', { name: 'Использовать резервный код' }));
    expect(screen.getByText(/Каждый работает один раз/)).toBeInTheDocument();
    await userEvent.type(screen.getByPlaceholderText('XXXX-XXXX'), 'AAAA-BBBB');
    await userEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));
    expect(await screen.findByText('резервный код неверный')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Код из приложения' }));
    expect(screen.getByPlaceholderText('000000')).toBeInTheDocument();
  });
});

describe('LoginPage passkey error', () => {
  it('показывает ошибку passkey', async () => {
    vi.mocked(startAuthentication).mockRejectedValue(new Error('denied'));
    stubFetch({
      '/api/login': { mfaRequired: true, passkey: true, totp: false },
      '/api/login/webauthn/begin': {},
    });
    renderUI(<LoginPage onLogin={vi.fn()} />);
    await userEvent.type(screen.getByPlaceholderText('Имя'), 'admin');
    await userEvent.type(screen.getByPlaceholderText('Пароль'), 'secret');
    await userEvent.click(screen.getByRole('button', { name: 'Войти' }));
    await userEvent.click(await screen.findByRole('button', { name: 'Войти с passkey' }));
    expect(await screen.findByText('passkey не сработал')).toBeInTheDocument();
    const close = document.querySelector('.g-alert__close-btn') as HTMLElement;
    await userEvent.click(close);
  });
});
