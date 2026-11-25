import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import AccountPage from './AccountPage';
import AdminsPage from './AdminsPage';
import AuditPage from './AuditPage';
import NewUserPage from './NewUserPage';
import { me } from '../test/data';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';

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
  {
    name: 'watcher',
    role: 'viewer',
    disabled: true,
    createdAt: '2026-11-02T00:00:00Z',
    passwordChangedAt: '2026-11-02T00:00:00Z',
    sessions: 0,
    totpEnabled: false,
  },
];

describe('AdminsPage', () => {
  it('рисует таблицу, сортирует и открывает создание', async () => {
    stubFetch({ '/api/admins': admins });
    renderUI(<AdminsPage me={me} />);
    expect(await screen.findByText('watcher')).toBeInTheDocument();
    await userEvent.click(screen.getAllByRole('button', { name: /Имя/ })[0]);
    expect(screen.getByText('↑')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Создать' }));
    expect(await screen.findByRole('dialog')).toBeInTheDocument();
  });
});

describe('AuditPage', () => {
  it('сортирует и фильтрует аудит', async () => {
    const event = {
      at: new Date().toISOString(),
      actor: 'admin',
      action: 'user.create',
      target: 'main',
      result: 'ok',
      ip: '192.0.2.1',
    };
    stubFetch({ '/api/audit': [event] });
    renderUI(<AuditPage />);
    expect(await screen.findByText('user.create')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: /^Кто/ }));
    expect(screen.getAllByText(/↑|↓/).length).toBeGreaterThan(0);
    await userEvent.selectOptions(screen.getByRole('combobox'), 'user.create');
    await userEvent.type(screen.getByRole('textbox'), 'main');
  });
});

describe('AccountPage', () => {
  it('показывает сессии и меняет пароль', async () => {
    stubFetch({
      '/api/account/sessions': [
        {
          id: 'a',
          userAgent: 'Safari',
          ip: '192.0.2.1',
          createdAt: new Date().toISOString(),
          current: false,
        },
      ],
      '/api/account/totp': { available: true, enabled: false, recoveryLeft: 0 },
      '/api/account/password': {},
      '/api/account/sessions/a/revoke': {},
    });
    renderUI(<AccountPage />);
    expect(await screen.findByText('Safari')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Завершить' }));
    await userEvent.type(screen.getByPlaceholderText('Текущий пароль'), 'old password 123');
    await userEvent.type(screen.getByPlaceholderText('Новый пароль (не короче 12)'), 'new password 1234');
    await userEvent.click(screen.getByRole('button', { name: 'Сменить' }));
    await waitFor(() => expect(screen.getByText(/Пароль изменён/)).toBeInTheDocument());
  });

  it('включает 2FA и показывает резервные коды', async () => {
    stubFetch({
      '/api/account/sessions': [],
      '/api/account/totp': { available: true, enabled: false, recoveryLeft: 0 },
      '/api/account/totp/setup': {
        secret: 'JBSWY3DPEHPK3PXP',
        url: 'otpauth://totp/x',
        qr: 'data:image/png;base64,AAAA',
      },
      '/api/account/totp/enable': { recoveryCodes: ['AAAA-BBBB'] },
    });
    renderUI(<AccountPage />);
    await userEvent.click(await screen.findByRole('button', { name: 'Включить 2FA' }));
    expect(await screen.findByText('JBSWY3DPEHPK3PXP')).toBeInTheDocument();
    await userEvent.type(screen.getByPlaceholderText('Код из приложения'), '123456');
    await userEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));
    expect(await screen.findByText('AAAA-BBBB')).toBeInTheDocument();
  });
});

describe('NewUserPage', () => {
  it('создаёт клиента и показывает ошибку для viewer', async () => {
    const fetchMock = stubFetch({
      '/api/users': { username: 'newclient' },
    });
    renderUI(<NewUserPage me={me} />);
    await userEvent.type(screen.getByPlaceholderText('Имя'), 'newclient');
    await userEvent.click(screen.getByRole('button', { name: 'Создать' }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    expect(screen.queryByText('Нет прав')).not.toBeInTheDocument();

    renderUI(<NewUserPage me={{ ...me, user: { name: 'viewer', role: 'viewer' } }} />);
    expect(screen.getByText('Нет прав')).toBeInTheDocument();
  });
});
