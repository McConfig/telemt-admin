import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import ConfigExchange from './ConfigExchange';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';
import { api, setCSRF } from '../api';

function calls(method: string, part: string) {
  return vi
    .mocked(fetch)
    .mock.calls.filter(
      ([url, init]) =>
        String(url).includes(part) && (init as RequestInit | undefined)?.method === method,
    );
}

const plan = {
  id: 'p1',
  fileName: 'x.yaml',
  blocks: [{ id: 'settings', title: 'Системные настройки', changes: 1, warnings: [] }],
  diff: { settings: [{ path: 'risk.warn_ips', before: 3, after: 4 }] },
  conflicts: { users: [{ name: 'demo', action: 'keep', changes: ['role'] }] },
  deletions: {},
  expiresAt: '2026-11-27T15:00:00Z',
};

describe('ConfigExchange', () => {
  it('экспортирует выбранные блоки', async () => {
    setCSRF('csrf');
    URL.createObjectURL = vi.fn(() => 'blob:x');
    URL.revokeObjectURL = vi.fn();
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(new Response(new Blob(['yaml']), { status: 200 })));
    vi.stubGlobal('fetch', fetchMock);
    renderUI(<ConfigExchange />);
    await userEvent.click(screen.getByRole('button', { name: 'Скачать YAML' }));
    await waitFor(() => expect(calls('POST', '/api/config/export')).toHaveLength(1));
    const init = calls('POST', '/api/config/export')[0][1] as RequestInit;
    expect(JSON.parse(String(init.body))).toMatchObject({ blocks: ['settings', 'limitTemplates'] });
    expect(await api.configBlocks()).toBeDefined();
  });

  it('проверяет файл и переключает конфликт', async () => {
    stubFetch({
      '/api/config/blocks': [{ id: 'settings', title: 'Системные настройки' }],
      '/api/config/precheck': plan,
      '/api/config/apply': { results: [{ block: 'settings', status: 'applied' }] },
    });
    renderUI(<ConfigExchange />);
    await screen.findByText('Обмен конфигурацией');
    const file = new File(['apiVersion: tgadmin.telemt/v1\n'], 'x.yaml', { type: 'text/yaml' });
    await userEvent.upload(screen.getByLabelText('YAML'), file);
    await userEvent.click(screen.getByRole('button', { name: 'Проверить' }));
    expect(await screen.findByText('risk.warn_ips: 3 → 4')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'переписать demo' }));
    await userEvent.click(screen.getByRole('button', { name: 'Применить выбранное' }));
    await waitFor(() => expect(calls('POST', '/api/config/apply')).toHaveLength(1));
    const init = calls('POST', '/api/config/apply')[0][1] as RequestInit;
    expect(JSON.parse(String(init.body))).toMatchObject({
      planId: 'p1',
      conflicts: { users: { demo: 'overwrite' } },
    });
  });
});

describe('ConfigExchange — дополнительные сценарии', () => {
  it('переключает пресеты, секреты и удаление отсутствующих', async () => {
    stubFetch({
      '/api/config/blocks': [
        { id: 'settings', title: 'Системные настройки' },
        { id: 'clients', title: 'Клиенты telemt' },
      ],
      '/api/config/precheck': plan,
    });
    renderUI(<ConfigExchange />);
    await screen.findByText('Обмен конфигурацией');
    await userEvent.click(screen.getByRole('button', { name: 'Полная выгрузка' }));
    await userEvent.click(screen.getByRole('checkbox', { name: 'Включить секреты' }));
    await userEvent.type(screen.getByLabelText('Пароль для секретов'), 'secret');
    await userEvent.type(screen.getByLabelText('Код для секретов'), '123456');
    await userEvent.upload(screen.getByLabelText('YAML'), new File(['yaml'], 'x.yaml'));
    await userEvent.click(screen.getByRole('button', { name: 'Проверить' }));
    await screen.findByText('risk.warn_ips: 3 → 4');
    await userEvent.click(screen.getByRole('checkbox', { name: 'Удалить отсутствующие' }));
    await userEvent.type(screen.getByLabelText('Имя установки'), 'demo');
    expect(screen.getByRole('button', { name: 'Применить выбранное' })).toBeEnabled();
  });

  it('показывает ошибку экспорта', async () => {
    stubFetch({
      '/api/config/blocks': [],
      '/api/config/export': () =>
        new Response(JSON.stringify({ error: { code: 'proxy_unavailable', message: 'нет связи' } }), {
          status: 400,
          headers: { 'Content-Type': 'application/json' },
        }),
    });
    renderUI(<ConfigExchange />);
    await screen.findByText('Обмен конфигурацией');
    await userEvent.click(screen.getByRole('button', { name: 'Скачать YAML' }));
    expect(await screen.findByText('нет связи')).toBeInTheDocument();
  });

  it('показывает пароли и ссылки после применения', async () => {
    stubFetch({
      '/api/config/blocks': [{ id: 'settings', title: 'Системные настройки' }],
      '/api/config/precheck': plan,
      '/api/config/apply': {
        results: [
          {
            block: 'users',
            status: 'applied',
            passwords: { demo: 'temp-pass' },
            links: { main: ['tg://proxy?server=demo.example.com'] },
          },
        ],
      },
    });
    renderUI(<ConfigExchange />);
    await screen.findByText('Обмен конфигурацией');
    await userEvent.upload(screen.getByLabelText('YAML'), new File(['yaml'], 'x.yaml'));
    await userEvent.click(screen.getByRole('button', { name: 'Проверить' }));
    await screen.findByText('risk.warn_ips: 3 → 4');
    await userEvent.click(screen.getByRole('button', { name: 'Применить выбранное' }));
    expect(await screen.findByText(/demo: временный пароль temp-pass/)).toBeInTheDocument();
    expect(await screen.findByText(/main: tg:\/\/proxy/)).toBeInTheDocument();
  });
});

describe('ConfigExchange — выбор блоков плана', () => {
  it('снимает блок плана и оставляет конфликт без изменений', async () => {
    stubFetch({
      '/api/config/blocks': [{ id: 'settings', title: 'Системные настройки' }],
      '/api/config/precheck': plan,
      '/api/config/apply': { results: [], error: 'частичная ошибка' },
    });
    renderUI(<ConfigExchange />);
    await screen.findByText('Обмен конфигурацией');
    await userEvent.upload(screen.getByLabelText('YAML'), new File(['yaml'], 'x.yaml'));
    await userEvent.click(screen.getByRole('button', { name: 'Проверить' }));
    await screen.findByText('risk.warn_ips: 3 → 4');
    const planChecks = screen.getAllByRole('checkbox', { name: 'Системные настройки' });
    await userEvent.click(planChecks[planChecks.length - 1]);
    await userEvent.click(planChecks[planChecks.length - 1]);
    await userEvent.click(screen.getByRole('button', { name: 'оставить demo' }));
    await userEvent.click(screen.getByRole('button', { name: 'Применить выбранное' }));
    expect(await screen.findByText('частичная ошибка')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('checkbox', { name: 'Включить секреты' }));
    await userEvent.click(screen.getByRole('checkbox', { name: 'Включить секреты' }));
  });
});
