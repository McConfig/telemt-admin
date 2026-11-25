import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import UsersPage from './UsersPage';
import { me, proxyUser, secondUser, summary } from '../test/data';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';

describe('UsersPage', () => {
  it('показывает сводку, ищет, сортирует и выполняет массовое действие', async () => {
    const fetchMock = stubFetch({
      '/api/users/main/disable': proxyUser,
      '/api/users/chernyh/disable': secondUser,
      '/api/users': [proxyUser, secondUser],
      '/api/summary': summary,
    });
    renderUI(<UsersPage me={me} />);

    expect(await screen.findByRole('link', { name: 'main' })).toBeInTheDocument();
    expect(screen.getByText('Клиентов')).toBeInTheDocument();

    const search = screen.getByPlaceholderText('Поиск по имени, стране, оператору');
    await userEvent.type(search, 'LeaseWeb');
    await waitFor(() => expect(screen.queryByRole('link', { name: 'main' })).not.toBeInTheDocument());

    await userEvent.clear(search);
    await userEvent.click(screen.getByRole('button', { name: /Трафик/ }));
    await userEvent.click(screen.getByRole('button', { name: /Статус/ }));
    expect(screen.getAllByText(/↑|↓/).length).toBeGreaterThanOrEqual(2);

    await userEvent.click(screen.getByRole('checkbox', { name: 'Выбрать main' }));
    expect(screen.getByText('Выбрано: 1')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Отключить' }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
  });

  it('фильтр только с иностранными IP', async () => {
    stubFetch({ '/api/users': [proxyUser, secondUser], '/api/summary': summary });
    renderUI(<UsersPage me={me} />);
    await screen.findByRole('link', { name: 'main' });
    await userEvent.click(screen.getByText('только с иностранными IP'));
    await waitFor(() => expect(screen.queryByRole('link', { name: 'main' })).not.toBeInTheDocument());
    expect(screen.getByRole('link', { name: 'chernyh' })).toBeInTheDocument();
  });
});
