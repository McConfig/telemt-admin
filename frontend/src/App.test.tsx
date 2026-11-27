import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it } from 'vitest';
import App from './App';
import { me, proxyUser, secondUser, summary } from './test/data';
import { stubFetch } from './test/stubFetch';

const admins = [
  {
    name: 'admin',
    role: 'admin',
    disabled: false,
    createdAt: '2026-11-01T00:00:00Z',
    passwordChangedAt: '2026-11-01T00:00:00Z',
    sessions: 1,
    totpEnabled: true,
  },
];

const diagnostics = {
  status: 'ok',
  checkedAt: new Date().toISOString(),
  version: 'v1.5.1',
  uptime: '1h',
  telemt: { url: 'http://telemt:9091', ok: true, latencyMs: 3 },
  collector: { up: true, lastSuccess: new Date().toISOString() },
  geoip: { loaded: true, built: new Date().toISOString(), ageDays: 1 },
  valkey: { ok: true, latencyMs: 1, keys: 10 },
  webhooks: { configured: false, events: [], secretSet: false, sent: 0, failed: 0 },
  backups: {
    valkey: { saveInProgress: false, lastStatus: 'ok', keys: 10 },
    host: { available: false },
  },
  security: {
    require2fa: false,
    graceDays: 0,
    daysLeft: 0,
    totpKeySet: true,
    adminsWithoutTotp: [],
  },
  warnings: [],
};

function baseRoutes() {
  return {
    '/api/meta': { version: 'v1.0.0', repo: 'https://example.com/repo' },
    '/api/logout': {},
    '/api/users': [proxyUser, secondUser],
    '/api/summary': summary,
    '/api/audit': [],
    '/api/admins': admins,
    '/api/account/sessions': [],
    '/api/account/totp': { available: true, enabled: false, recoveryLeft: 0 },
    '/api/settings/templates': { default: '', templates: [] },
    '/api/settings': { version: 0, sections: [] },
    '/api/settings/backups': {
      valkey: { saveInProgress: false, keys: 0 },
      host: { available: false },
    },
    '/api/settings/system': {
      version: 'v1.0.0',
      uptime: '1h',
      valkey: { keys: 0 },
      counts: { uiUsers: 1, auditEvents: 0, templates: 0 },
    },
    '/api/settings/history': [],
    '/api/diagnostics': diagnostics,
    '/api/users/main': proxyUser,
    '/api/users/main/traffic': [],
    '/api/users/main/timeline': [],
    '/api/users/main/trusted': { countries: [], ips: [] },
  };
}

async function renderApp(path: string, routes: Record<string, unknown> = {}) {
  window.history.pushState({}, '', path);
  stubFetch({ ...baseRoutes(), ...routes });
  const view = render(<App />);
  return view;
}

beforeEach(() => {
  localStorage.clear();
  delete document.documentElement.dataset.theme;
});

describe('App', () => {
  it('загружает me, показывает список, переключает тему и выходит', async () => {
    await renderApp('/', { '/api/me': me });
    expect(await screen.findByText('Пользователи telemt')).toBeInTheDocument();
    expect(localStorage.getItem('tgadmin-theme')).toBe('system');

    await userEvent.click(screen.getByText('admin'));
    await userEvent.click(await screen.findByRole('combobox', { name: 'Тема' }));
    await userEvent.click(screen.getByRole('option', { name: 'Светлая' }));
    await waitFor(() => expect(localStorage.getItem('tgadmin-theme')).toBe('light'));
    expect(document.body.className).toContain('g-root_theme_light');

    await userEvent.click(screen.getByRole('button', { name: 'Выйти' }));
    expect(await screen.findByText('Вход в TeleMT Admin')).toBeInTheDocument();
  });

  it('по умолчанию использует системную тему', async () => {
    await renderApp('/', { '/api/me': me });
    expect(await screen.findByText('Пользователи telemt')).toBeInTheDocument();
    expect(localStorage.getItem('tgadmin-theme')).toBe('system');
    expect(document.body.className).toContain('g-root_theme_light');
  });

  it('стартует со светлой темы из localStorage', async () => {
    localStorage.setItem('tgadmin-theme', 'light');
    await renderApp('/', { '/api/me': me });
    expect(await screen.findByText('Пользователи telemt')).toBeInTheDocument();
    expect(document.body.className).toContain('g-root_theme_light');
    await userEvent.click(screen.getByText('admin'));
    await userEvent.click(await screen.findByRole('combobox', { name: 'Тема' }));
    await userEvent.click(screen.getByRole('option', { name: 'Тёмная' }));
    await waitFor(() => expect(localStorage.getItem('tgadmin-theme')).toBe('dark'));
  });

  it('показывает форму входа после 401 и пускает по паролю', async () => {
    await renderApp('/', {
      '/api/me': () => new Response('', { status: 401 }),
      '/api/login': me,
    });
    expect(await screen.findByText('Вход в TeleMT Admin')).toBeInTheDocument();
    await userEvent.type(screen.getByPlaceholderText('Имя'), 'admin');
    await userEvent.type(screen.getByPlaceholderText('Пароль'), 'secret');
    await userEvent.click(screen.getByRole('button', { name: 'Войти' }));
    expect(await screen.findByText('Пользователи telemt')).toBeInTheDocument();
  });

  it('возвращает на вход по событию unauthorized', async () => {
    await renderApp('/', { '/api/me': me });
    expect(await screen.findByText('Пользователи telemt')).toBeInTheDocument();
    window.dispatchEvent(new CustomEvent('tgadmin:unauthorized'));
    expect(await screen.findByText('Вход в TeleMT Admin')).toBeInTheDocument();
  });

  it('ограничивает интерфейс при истёкшем сроке 2FA и разлогинивает', async () => {
    await renderApp('/', {
      '/api/me': {
        ...me,
        require2fa: { enabled: true, enforced: true, daysLeft: 0 },
      },
      '/api/logout': {},
    });
    expect(await screen.findByText('Требуется двухфакторная аутентификация')).toBeInTheDocument();
    expect(screen.getByText('Аккаунт')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Выйти' }));
    expect(await screen.findByText('Вход в TeleMT Admin')).toBeInTheDocument();
  });

  it('показывает предупреждение о льготном сроке 2FA', async () => {
    await renderApp('/', {
      '/api/me': { ...me, require2fa: { enabled: true, enforced: false, daysLeft: 3 } },
    });
    expect(await screen.findByText(/Через 3 дн/)).toBeInTheDocument();
  });

  it('предупреждает, что срок 2FA истекает сегодня', async () => {
    await renderApp('/', {
      '/api/me': { ...me, require2fa: { enabled: true, enforced: false, daysLeft: 0 } },
    });
    expect(await screen.findByText(/Сегодня истекает льготный срок/)).toBeInTheDocument();
  });

  it('открывает страницу клиента по маршруту', async () => {
    await renderApp('/users/main', { '/api/me': me });
    expect(await screen.findByText('main')).toBeInTheDocument();
    expect(await screen.findByText('Трафик по дням')).toBeInTheDocument();
  });

  it('открывает создание клиента по маршруту', async () => {
    await renderApp('/users/new', { '/api/me': me });
    expect(await screen.findByText('Новый пользователь')).toBeInTheDocument();
  });

  it('открывает аудит по маршруту', async () => {
    await renderApp('/audit', { '/api/me': me });
    expect(await screen.findByRole('link', { name: 'Экспорт CSV' })).toBeInTheDocument();
  });

  it('открывает аккаунт по маршруту', async () => {
    await renderApp('/account', { '/api/me': me });
    expect(await screen.findByText('Смена пароля')).toBeInTheDocument();
  });

  it('открывает пользователей админки по маршруту', async () => {
    await renderApp('/admins', { '/api/me': me });
    expect(await screen.findByText('Пользователи админки')).toBeInTheDocument();
  });

  it('открывает настройки и диагностику для администратора', async () => {
    const view = await renderApp('/settings', { '/api/me': me });
    expect(await screen.findByText('Настройки')).toBeInTheDocument();
    view.unmount();

    await renderApp('/diagnostics', { '/api/me': me });
    expect(await screen.findByText('Диагностика')).toBeInTheDocument();
  });

  it('перенаправляет viewer из настроек и неизвестного маршрута на список', async () => {
    const viewer = { ...me, user: { name: 'viewer', role: 'viewer' as const } };
    const view = await renderApp('/settings', { '/api/me': viewer });
    expect(await screen.findByText('Пользователи telemt')).toBeInTheDocument();
    view.unmount();

    await renderApp('/no-such-page', { '/api/me': me });
    expect(await screen.findByText('Пользователи telemt')).toBeInTheDocument();
  });
});
