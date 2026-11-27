import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import Layout from './Layout';
import { me } from './test/data';
import { renderUI } from './test/render';
import { stubFetch } from './test/stubFetch';

describe('Layout', () => {
  it('показывает предупреждение о сроке 2FA', () => {
    stubFetch({ '/api/meta': { version: 'v9.9.9', repo: 'https://example.com/repo' } });
    renderUI(
      <Layout
        me={{ ...me, require2fa: { enabled: true, enforced: false, daysLeft: 3 } }}
        onLogout={() => {}}
        theme="system"
        onThemeChange={() => {}}
      >
        <div>содержимое</div>
      </Layout>,
    );
    expect(screen.getByText(/Через 3 дн/)).toBeInTheDocument();
  });

  it('показывает меню, тему, аккаунт, выход и футер', async () => {
    stubFetch({ '/api/meta': { version: 'v9.9.9', repo: 'https://example.com/repo' } });
    const onLogout = vi.fn();
    renderUI(
      <Layout me={me} onLogout={onLogout} theme="system" onThemeChange={() => {}}>
        <div>содержимое</div>
      </Layout>,
    );
    expect(screen.getByText('содержимое')).toBeInTheDocument();
    expect(screen.getByText('Пользователи UI')).toBeInTheDocument();
    expect(screen.getByText('Сервисные аккаунты')).toBeInTheDocument();
    expect(screen.getByText('Настройки')).toBeInTheDocument();
    expect(screen.getByText('Диагностика')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Выйти' }));
    expect(onLogout).toHaveBeenCalled();
    expect(await screen.findByText(/v9.9.9/)).toBeInTheDocument();
  });

  it('скрывает админские пункты у viewer', () => {
    stubFetch({ '/api/meta': { version: 'v9.9.9', repo: 'https://example.com/repo' } });
    renderUI(
      <Layout
        me={{ ...me, user: { ...me.user, role: 'viewer' } }}
        onLogout={() => {}}
        theme="system"
        onThemeChange={() => {}}
      >
        <div>содержимое</div>
      </Layout>,
    );
    expect(screen.queryByText('Пользователи UI')).not.toBeInTheDocument();
    expect(screen.queryByText('Сервисные аккаунты')).not.toBeInTheDocument();
    expect(screen.queryByText('Настройки')).not.toBeInTheDocument();
  });

  it('на мобильном принудительно сворачивает сайдбар', () => {
    localStorage.setItem('tgadmin-sidebar-compact', 'false');
    vi.spyOn(window, 'matchMedia').mockReturnValue({
      matches: true,
      media: '(max-width: 960px)',
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    } as unknown as MediaQueryList);
    stubFetch({ '/api/meta': { version: 'v9.9.9', repo: 'https://example.com/repo' } });
    renderUI(
      <Layout me={me} onLogout={() => {}} theme="system" onThemeChange={() => {}}>
        <div>содержимое</div>
      </Layout>,
    );
    expect(screen.queryByRole('combobox', { name: 'Тема' })).not.toBeInTheDocument();
  });

  it('переключает тему на контрастную', async () => {
    stubFetch({ '/api/meta': { version: 'v9.9.9', repo: 'https://example.com/repo' } });
    const onThemeChange = vi.fn();
    renderUI(
      <Layout me={me} onLogout={() => {}} theme="dark" onThemeChange={onThemeChange}>
        <div>содержимое</div>
      </Layout>,
    );
    await userEvent.click(screen.getByRole('combobox', { name: 'Тема' }));
    await userEvent.click(screen.getByRole('option', { name: 'Тёмная контрастная' }));
    expect(onThemeChange).toHaveBeenCalledWith('dark-hc');
  });
});
