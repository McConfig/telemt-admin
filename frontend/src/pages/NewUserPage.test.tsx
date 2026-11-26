import { fireEvent, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { Route, Routes } from 'react-router-dom';
import NewUserPage from './NewUserPage';
import { me } from '../test/data';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';

const template = {
  name: 'router',
  maxIps: 5,
  maxTcpConns: 20,
  dataQuotaBytes: 10 * 1024 ** 3,
  quotaPeriod: 'monthly',
  rateLimitUpBps: 1000,
  rateLimitDownBps: 2000,
  expirationDays: 7,
  sourceDeny: ['203.0.113.0/24'],
};

const phone = {
  name: 'phone',
  maxIps: 2,
  maxTcpConns: 5,
};

function renderNewUser() {
  renderUI(
    <Routes>
      <Route path="/users/new" element={<NewUserPage me={me} />} />
      <Route path="/users/:name" element={<div>карточка клиента</div>} />
    </Routes>,
    '/users/new',
  );
}

function createCall() {
  return vi
    .mocked(fetch)
    .mock.calls.find(
      ([url, init]) =>
        String(url).endsWith('/api/users') && (init as RequestInit | undefined)?.method === 'POST',
    );
}

describe('NewUserPage', () => {
  it('подставляет шаблон по умолчанию и переключает шаблоны', async () => {
    stubFetch({
      '/api/settings/templates': { default: 'router', templates: [template, phone] },
    });
    renderNewUser();

    await screen.findByText('Шаблон лимитов');
    expect(screen.getByPlaceholderText('Максимум уникальных IP')).toHaveValue('5');
    expect(screen.getByPlaceholderText('Максимум соединений')).toHaveValue('20');
    expect(screen.getByPlaceholderText('Квота, ГиБ')).toHaveValue('10.00');
    expect(screen.getByPlaceholderText('Скорость вверх, Кбит/с')).toHaveValue('1');
    expect(screen.getByPlaceholderText('Скорость вниз, Кбит/с')).toHaveValue('2');
    expect(screen.getByPlaceholderText('203.0.113.0/24')).toHaveValue('203.0.113.0/24');

    const select = screen.getAllByRole('combobox')[0];
    await userEvent.selectOptions(select, 'phone');
    expect(screen.getByPlaceholderText('Максимум уникальных IP')).toHaveValue('2');
    expect(screen.getByPlaceholderText('Максимум соединений')).toHaveValue('5');
    expect(screen.getByPlaceholderText('Квота, ГиБ')).toHaveValue('');
  });

  it('создаёт клиента со всеми полями и переходит на карточку', async () => {
    stubFetch({
      '/api/settings/templates': { default: 'router', templates: [template] },
      '/api/users': { username: 'newclient' },
    });
    renderNewUser();
    await screen.findByText('Шаблон лимитов');

    await userEvent.type(screen.getByPlaceholderText('Имя'), '  newclient  ');
    await userEvent.type(
      screen.getByPlaceholderText('Секрет (32 hex, пусто — сгенерировать)'),
      'aabbccdd',
    );
    const maxIps = screen.getByPlaceholderText('Максимум уникальных IP');
    await userEvent.clear(maxIps);
    await userEvent.type(maxIps, '6');
    await userEvent.selectOptions(screen.getByDisplayValue('месяц'), 'weekly');
    const quota = screen.getByPlaceholderText('Квота, ГиБ');
    await userEvent.clear(quota);
    await userEvent.type(quota, '1.5');
    await userEvent.clear(screen.getByPlaceholderText('Скорость вверх, Кбит/с'));
    await userEvent.type(screen.getByPlaceholderText('Скорость вверх, Кбит/с'), '100');
    fireEvent.change(screen.getByLabelText('Действует до'), {
      target: { value: '2026-12-31T10:00' },
    });
    fireEvent.change(screen.getByPlaceholderText('203.0.113.0/24'), {
      target: { value: '198.51.100.0/24, 192.0.2.0/24\n203.0.113.0/24' },
    });

    await userEvent.click(screen.getByRole('button', { name: 'Создать' }));
    expect(await screen.findByText('карточка клиента')).toBeInTheDocument();

    await waitFor(() => expect(createCall()).toBeTruthy());
    const body = JSON.parse(String((createCall()?.[1] as RequestInit).body));
    expect(body).toMatchObject({
      name: 'newclient',
      secret: 'aabbccdd',
      maxUniqueIps: 6,
      maxTcpConns: 20,
      dataQuotaBytes: Math.round(1.5 * 1024 ** 3),
      quotaPeriod: 'weekly',
      rateLimitUpBps: 100_000,
      rateLimitDownBps: 2000,
      sourceDeny: ['198.51.100.0/24', '192.0.2.0/24', '203.0.113.0/24'],
    });
    expect(body.expiration).toContain('2026-12-31');
  });

  it('показывает ошибку создания и работает без шаблонов', async () => {
    stubFetch({
      '/api/settings/templates': () => new Response('', { status: 500 }),
      '/api/users': () =>
        new Response(JSON.stringify({ error: { code: 'conflict', message: 'имя занято' } }), {
          status: 409,
          headers: { 'Content-Type': 'application/json' },
        }),
    });
    renderNewUser();
    expect(screen.queryByText('Шаблон лимитов')).not.toBeInTheDocument();
    await userEvent.type(screen.getByPlaceholderText('Имя'), 'dup');
    await userEvent.click(screen.getByRole('button', { name: 'Создать' }));
    expect(await screen.findByText('имя занято')).toBeInTheDocument();
  });

  it('возвращает к списку по кнопке Отмена', async () => {
    stubFetch({ '/api/settings/templates': { default: '', templates: [] } });
    renderUI(
      <Routes>
        <Route path="/users/new" element={<NewUserPage me={me} />} />
        <Route path="/" element={<div>список клиентов</div>} />
      </Routes>,
      '/users/new',
    );
    await userEvent.click(await screen.findByRole('button', { name: 'Отмена' }));
    expect(await screen.findByText('список клиентов')).toBeInTheDocument();
  });

  it('закрывает раздел для viewer', () => {
    stubFetch({});
    renderUI(<NewUserPage me={{ ...me, user: { name: 'viewer', role: 'viewer' } }} />);
    expect(screen.getByText('Нет прав')).toBeInTheDocument();
  });
});
