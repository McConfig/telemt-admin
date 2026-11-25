import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { Route, Routes } from 'react-router-dom';
import UserPage from './UserPage';
import { me, proxyUser } from '../test/data';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';

function renderUser() {
  stubFetch({
    '/api/users/main/traffic': [{ day: '2026-11-24', bytes: 1024 }],
    '/api/users/main/timeline': [
      { day: '2026-11-24', newIps: 1, maxIps: 2, bytes: 1024 },
    ],
    '/api/users/main/trusted': { countries: [], ips: [] },
    '/api/users/main/note': { note: 'телефон' },
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
