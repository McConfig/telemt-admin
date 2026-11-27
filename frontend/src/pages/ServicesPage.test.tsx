import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import ServicesPage from './ServicesPage';
import { me } from '../test/data';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';
import { chooseSelect } from '../test/select';
import type { ServiceAccount } from '../api';

const service: ServiceAccount = {
  name: 'monitoring',
  description: 'Prometheus',
  scope: 'read',
  disabled: false,
  tokens: 1,
  createdAt: '2026-11-26T10:00:00Z',
  updatedAt: '2026-11-26T10:00:00Z',
  createdBy: 'admin',
};

describe('ServicesPage', () => {
  it('показывает список и создаёт сервис', async () => {
    let items = [service];
    stubFetch({
      '/api/services': (_url: URL, init?: RequestInit) => {
        if (init?.method === 'POST') {
          const body = JSON.parse(String(init.body)) as {
            name: string;
            description: string;
            scope: ServiceAccount['scope'];
          };
          const created: ServiceAccount = {
            ...service,
            name: body.name,
            description: body.description,
            scope: body.scope,
            tokens: 0,
          };
          items = [...items, created];
          return created;
        }
        return items;
      },
    });
    renderUI(<ServicesPage me={me} />);
    expect(await screen.findByText('monitoring')).toBeInTheDocument();
    expect(screen.getByText('Prometheus')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Токены monitoring' }));
    expect(await screen.findByText('Токены: monitoring')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Закрыть' }));

    await userEvent.click(screen.getByRole('button', { name: 'Создать сервис' }));
    await userEvent.type(screen.getByPlaceholderText('Имя'), 'ci');
    await userEvent.type(screen.getByPlaceholderText('Описание'), 'CI runner');
    await chooseSelect('Права', 'Клиенты — управление клиентами');
    await userEvent.click(screen.getByRole('button', { name: 'Создать' }));

    expect(await screen.findByText('ci')).toBeInTheDocument();
    expect(screen.getByText('CI runner')).toBeInTheDocument();
  });

  it('редактирует, отключает и удаляет сервис', async () => {
    let items = [service];
    stubFetch({
      '/api/services': (_url: URL, init?: RequestInit) => {
        if (init?.method === 'PATCH') {
          const body = JSON.parse(String(init.body)) as Partial<ServiceAccount>;
          items = items.map((item) =>
            item.name === 'monitoring' ? { ...item, ...body } : item,
          );
          return items.find((item) => item.name === 'monitoring');
        }
        if (init?.method === 'DELETE') {
          items = items.filter((item) => item.name !== 'monitoring');
          return new Response(null, { status: 204 });
        }
        return items;
      },
    });
    renderUI(<ServicesPage me={me} />);
    await screen.findByText('monitoring');

    await userEvent.click(screen.getByRole('button', { name: 'Изменить monitoring' }));
    let edit = await screen.findByRole('dialog');
    await userEvent.click(within(edit).getByRole('button', { name: 'Отмена' }));
    await userEvent.click(screen.getByRole('button', { name: 'Изменить monitoring' }));
    edit = await screen.findByRole('dialog');
    await userEvent.clear(within(edit).getByPlaceholderText('Описание'));
    await userEvent.type(within(edit).getByPlaceholderText('Описание'), 'Новое описание');
    await chooseSelect('Права', 'Администратор — полный доступ');
    await userEvent.click(within(edit).getByRole('button', { name: 'Сохранить' }));
    expect(await screen.findByText('Новое описание')).toBeInTheDocument();

    await userEvent.click(await screen.findByRole('button', { name: 'Отключить monitoring' }));
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Включить monitoring' })).toBeInTheDocument(),
    );

    await userEvent.click(screen.getByRole('button', { name: 'Удалить monitoring' }));
    const del = await screen.findByRole('dialog');
    await userEvent.type(within(del).getByPlaceholderText('Имя сервиса'), 'monitoring');
    await userEvent.click(within(del).getByRole('button', { name: 'Удалить' }));
    await waitFor(() => expect(screen.queryByText('monitoring')).not.toBeInTheDocument());
  });
});
