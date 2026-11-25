import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
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
