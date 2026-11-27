import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import ProxyPanel from './ProxyPanel';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';

function calls(method: string, part: string) {
  return vi
    .mocked(fetch)
    .mock.calls.filter(
      ([url, init]) =>
        String(url).includes(part) && (init as RequestInit | undefined)?.method === method,
    );
}

const schema = {
  version: 1,
  fields: [
    {
      path: 'general.links.public_host',
      value: { type: 'string' },
      api: { config_read: true, config_write: true },
      apply: { watcher: 'supported', reload: 'supported', effect: 'links' },
    },
    {
      path: 'server.port',
      value: { type: 'integer' },
      api: { config_read: false, config_write: false },
      apply: { watcher: 'unsupported', reload: 'unsupported', effect: 'restart' },
      non_editable_reason: 'process_owned',
    },
  ],
};

describe('ProxyPanel', () => {
  it('правит вложенный путь и применяет патч', async () => {
    stubFetch({
      '/api/proxy/schema': schema,
      '/api/proxy/config': {
        revision: 'rev1',
        config: { general: { links: { public_host: 'old.example' } } },
      },
      '/api/proxy/validate': { ok: true, changedPaths: [] },
      '/api/proxy/apply': { revision: 'rev2', changedPaths: ['general.links.public_host'], reloadId: 7 },
      '/api/proxy/reload/7': { reload_id: 7, state: 'succeeded' },
    });
    renderUI(<ProxyPanel />);
    const input = await screen.findByLabelText('general.links.public_host');
    await userEvent.clear(input);
    await userEvent.type(input, 'new.example');
    await userEvent.click(screen.getByRole('checkbox', { name: 'Подтвердить опасные изменения' }));
    await userEvent.click(screen.getByRole('button', { name: 'Применить' }));
    await waitFor(() => expect(calls('POST', '/api/proxy/apply')).toHaveLength(1));
    const init = calls('POST', '/api/proxy/apply')[0][1] as RequestInit;
    expect(JSON.parse(String(init.body))).toMatchObject({
      patch: { general: { links: { public_host: 'new.example' } } },
      confirm: true,
    });
    expect(await screen.findByText('Перезагрузка 7: succeeded')).toBeInTheDocument();
  });

  it('показывает конфликт ревизии', async () => {
    stubFetch({
      '/api/proxy/schema': schema,
      '/api/proxy/config': { revision: 'rev1', config: { general: { links: { public_host: 'old' } } } },
      '/api/proxy/apply': () =>
        new Response(JSON.stringify({ error: { code: 'proxy_conflict', message: 'stale' } }), {
          status: 409,
          headers: { 'Content-Type': 'application/json' },
        }),
    });
    renderUI(<ProxyPanel />);
    const input = await screen.findByLabelText('general.links.public_host');
    await userEvent.type(input, 'x');
    await userEvent.click(screen.getByRole('checkbox', { name: 'Подтвердить опасные изменения' }));
    await userEvent.click(screen.getByRole('button', { name: 'Применить' }));
    expect(await screen.findByText('конфиг изменился, обновите страницу')).toBeInTheDocument();
  });
});

const extendedSchema = {
  version: 1,
  fields: [
    ...schema.fields,
    {
      path: 'censorship.mask',
      value: { type: 'boolean' },
      api: { config_read: true, config_write: true },
      apply: { watcher: 'supported', reload: 'supported', effect: 'mask' },
    },
    {
      path: 'general.links.public_port',
      value: { type: 'integer' },
      api: { config_read: true, config_write: true },
      apply: { watcher: 'supported', reload: 'supported', effect: 'links' },
    },
  ],
};

describe('ProxyPanel — дополнительные сценарии', () => {
  it('валидирует изменения и переключает булево поле', async () => {
    stubFetch({
      '/api/proxy/schema': extendedSchema,
      '/api/proxy/config': {
        revision: 'rev1',
        config: {
          general: { links: { public_host: 'old.example', public_port: 443 } },
          censorship: { mask: true },
        },
      },
      '/api/proxy/validate': { ok: true, changedPaths: ['censorship.mask'] },
    });
    renderUI(<ProxyPanel />);
    const mask = await screen.findByRole('checkbox', { name: 'censorship.mask' });
    await userEvent.click(mask);
    const port = screen.getByLabelText('general.links.public_port');
    await userEvent.clear(port);
    await userEvent.type(port, '8443');
    await userEvent.click(screen.getByRole('button', { name: 'Проверить' }));
    expect(await screen.findByText('Проверка пройдена')).toBeInTheDocument();
    expect(screen.getByText('Только через файл или перезапуск')).toBeInTheDocument();
  });

  it('показывает ошибку загрузки схемы', async () => {
    stubFetch({
      '/api/proxy/schema': () =>
        new Response(JSON.stringify({ error: { code: 'proxy_unavailable', message: 'нет связи' } }), {
          status: 503,
          headers: { 'Content-Type': 'application/json' },
        }),
      '/api/proxy/config': { revision: 'rev1', config: {} },
    });
    renderUI(<ProxyPanel />);
    expect(await screen.findByText('нет связи')).toBeInTheDocument();
  });
});

describe('ProxyPanel — ошибка применения', () => {
  it('показывает ошибку применения', async () => {
    stubFetch({
      '/api/proxy/schema': schema,
      '/api/proxy/config': { revision: 'rev1', config: { general: { links: { public_host: 'old' } } } },
      '/api/proxy/apply': () =>
        new Response(JSON.stringify({ error: { code: 'proxy_unavailable', message: 'нет связи' } }), {
          status: 503,
          headers: { 'Content-Type': 'application/json' },
        }),
    });
    renderUI(<ProxyPanel />);
    const input = await screen.findByLabelText('general.links.public_host');
    await userEvent.type(input, 'x');
    await userEvent.click(screen.getByRole('checkbox', { name: 'Подтвердить опасные изменения' }));
    await userEvent.click(screen.getByRole('button', { name: 'Применить' }));
    expect(await screen.findByText('нет связи')).toBeInTheDocument();
  });
});

const jsonSchema = {
  version: 1,
  fields: [
    {
      path: 'general.links.public_host',
      value: { type: 'string' },
      api: { config_read: true, config_write: true },
      apply: { watcher: 'supported', reload: 'supported', effect: 'links' },
    },
    {
      path: 'upstreams',
      value: { type: 'array', items: { type: 'object' } },
      api: { config_read: true, config_write: true },
      apply: { watcher: 'supported', reload: 'supported', effect: 'routing' },
    },
    {
      path: 'server.port',
      value: { type: 'integer' },
      api: { config_read: false, config_write: false },
      apply: { watcher: 'unsupported', reload: 'unsupported', effect: 'restart' },
      non_editable_reason: 'process_owned',
    },
  ],
};

describe('ProxyPanel — таблица и JSON', () => {
  it('показывает колонки и чекбокс для boolean', async () => {
    stubFetch({
      '/api/proxy/schema': extendedSchema,
      '/api/proxy/config': {
        revision: 'rev1',
        config: {
          general: { links: { public_host: 'old', public_port: 443 } },
          censorship: { mask: true },
        },
      },
    });
    renderUI(<ProxyPanel />);
    expect(await screen.findAllByText('Параметр')).toHaveLength(1);
    expect(screen.getAllByText('Значение').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Включено').length).toBeGreaterThan(0);
    expect(screen.getByRole('checkbox', { name: 'censorship.mask' })).toBeChecked();
  });

  it('редактирует JSON с валидацией и применяет массив', async () => {
    stubFetch({
      '/api/proxy/schema': jsonSchema,
      '/api/proxy/config': {
        revision: 'rev1',
        config: {
          general: { links: { public_host: 'old' } },
          upstreams: [{ address: '1.1.1.1:443' }],
        },
      },
      '/api/proxy/apply': { revision: 'rev2', changedPaths: ['upstreams'], reloadId: 0 },
      '/api/proxy/reload/0': { reload_id: 0, state: 'succeeded' },
    });
    renderUI(<ProxyPanel />);
    await userEvent.click(await screen.findByRole('button', { name: 'Изменить JSON' }));
    const dialog = await screen.findByRole('dialog');
    const editor = within(dialog).getByLabelText('JSON');
    fireEvent.change(editor, { target: { value: '{{' } });
    expect(await within(dialog).findByText(/Expected|ожидается|Unexpected|property name/)).toBeInTheDocument();
    fireEvent.change(editor, { target: { value: '[{"address":"2.2.2.2:443"}]' } });
    await within(dialog).getByRole('button', { name: 'Применить' }).click();
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    await userEvent.click(screen.getByRole('button', { name: 'Применить' }));
    await waitFor(() => expect(calls('POST', '/api/proxy/apply')).toHaveLength(1));
    const init = calls('POST', '/api/proxy/apply')[0][1] as RequestInit;
    expect(JSON.parse(String(init.body)).patch).toMatchObject({
      upstreams: [{ address: '2.2.2.2:443' }],
    });
  });

  it('фильтрует параметры поиском', async () => {
    stubFetch({
      '/api/proxy/schema': extendedSchema,
      '/api/proxy/config': {
        revision: 'rev1',
        config: {
          general: { links: { public_host: 'old', public_port: 443 } },
          censorship: { mask: true },
        },
      },
    });
    renderUI(<ProxyPanel />);
    await screen.findByText('general.links.public_host');
    await userEvent.type(screen.getByLabelText('Поиск по параметру'), 'censorship');
    expect(screen.queryByText('general.links.public_host')).not.toBeInTheDocument();
    expect(screen.getByText('censorship.mask')).toBeInTheDocument();
  });
});

const objectSchema = {
  version: 1,
  fields: [
    {
      path: 'dc_overrides',
      value: { type: 'object' },
      api: { config_read: true, config_write: true },
      apply: { watcher: 'supported', reload: 'supported', effect: 'routing' },
    },
    {
      path: 'general.log_level',
      value: { type: 'string', enum: ['info', 'debug'] },
      api: { config_read: true, config_write: true },
      apply: { watcher: 'supported', reload: 'supported', effect: 'logging' },
    },
    {
      path: 'server.metrics',
      value: { type: 'boolean' },
      api: { config_read: false, config_write: false },
      apply: { watcher: 'unsupported', reload: 'unsupported', effect: 'restart' },
      non_editable_reason: 'process_owned',
    },
  ],
};

describe('ProxyPanel — объект, enum и read-only', () => {
  it('валидирует тип объекта, форматирует и переключает enum', async () => {
    stubFetch({
      '/api/proxy/schema': objectSchema,
      '/api/proxy/config': {
        revision: 'rev1',
        config: { dc_overrides: { '203': ['1.1.1.1:443'] }, general: { log_level: 'info' } },
      },
    });
    renderUI(<ProxyPanel />);
    await userEvent.click(await screen.findByRole('button', { name: 'Изменить JSON' }));
    const dialog = await screen.findByRole('dialog');
    const editor = within(dialog).getByLabelText('JSON');
    fireEvent.change(editor, { target: { value: '[]' } });
    expect(await within(dialog).findByText('ожидается JSON-объект')).toBeInTheDocument();
    await within(dialog).getByRole('button', { name: 'Форматировать' }).click();
    fireEvent.change(editor, { target: { value: '{"204":["2.2.2.2:443"]}' } });
    await within(dialog).getByRole('button', { name: 'Применить' }).click();
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

    await userEvent.click(screen.getByLabelText('general.log_level'));
    await userEvent.click(await screen.findByText('debug'));
    expect(screen.getAllByText('general.log_level').length).toBeGreaterThan(0);
  });

  it('показывает неактивный чекбокс для read-only boolean', async () => {
    stubFetch({
      '/api/proxy/schema': objectSchema,
      '/api/proxy/config': { revision: 'rev1', config: { server: { metrics: true } } },
    });
    renderUI(<ProxyPanel />);
    await screen.findByText('Только через файл или перезапуск');
    await userEvent.click(screen.getByText('Только через файл или перезапуск'));
    const checkbox = await screen.findByRole('checkbox', { name: 'server.metrics' });
    expect(checkbox).toBeDisabled();
    expect(checkbox).toBeChecked();
  });
});

const helpSchema = {
  version: 1,
  fields: [
    {
      path: 'general.links.public_host',
      description: 'English description',
      value: { type: 'string' },
      api: { config_read: true, config_write: true },
      apply: { watcher: 'supported', reload: 'supported', effect: 'links' },
    },
    {
      path: 'censorship.mask',
      description: 'Маскировать неизвестные соединения',
      value: { type: 'boolean' },
      api: { config_read: true, config_write: true },
      apply: { watcher: 'supported', reload: 'supported', effect: 'mask' },
    },
  ],
};

describe('ProxyPanel — подсказки, сворачивание и подсветка', () => {
  it('показывает подсказку по вопросику', async () => {
    stubFetch({
      '/api/proxy/schema': helpSchema,
      '/api/proxy/config': {
        revision: 'rev1',
        config: { general: { links: { public_host: 'old' } }, censorship: { mask: true } },
      },
    });
    renderUI(<ProxyPanel />);
    await userEvent.click(await screen.findByRole('button', { name: 'Описание general.links.public_host' }));
    expect(await screen.findByText(/Переопределение общедоступного имени/)).toBeInTheDocument();
  });

  it('сворачивает и разворачивает группу', async () => {
    stubFetch({
      '/api/proxy/schema': helpSchema,
      '/api/proxy/config': {
        revision: 'rev1',
        config: { general: { links: { public_host: 'old' } }, censorship: { mask: true } },
      },
    });
    renderUI(<ProxyPanel />);
    await screen.findByText('general.links.public_host');
    await userEvent.click(screen.getByRole('button', { name: /^general/ }));
    expect(screen.queryByText('general.links.public_host')).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: /^general/ }));
    expect(await screen.findByText('general.links.public_host')).toBeInTheDocument();
  });

  it('подсвечивает рамку изменённого параметра', async () => {
    stubFetch({
      '/api/proxy/schema': helpSchema,
      '/api/proxy/config': {
        revision: 'rev1',
        config: { general: { links: { public_host: 'old' } }, censorship: { mask: true } },
      },
    });
    renderUI(<ProxyPanel />);
    const input = await screen.findByLabelText('general.links.public_host');
    expect(input.closest('.proxy-row')).not.toHaveClass('proxy-row_changed');
    await userEvent.type(input, 'x');
    expect(input.closest('.proxy-row')).toHaveClass('proxy-row_changed');
  });

  it('не рисует содержимое в колонке включения у не-boolean', async () => {
    stubFetch({
      '/api/proxy/schema': helpSchema,
      '/api/proxy/config': {
        revision: 'rev1',
        config: { general: { links: { public_host: 'old' } }, censorship: { mask: true } },
      },
    });
    renderUI(<ProxyPanel />);
    const input = await screen.findByLabelText('general.links.public_host');
    const row = input.closest('.proxy-row') as HTMLElement;
    expect(row.querySelectorAll('.proxy-cell')[2].textContent).toBe('');
  });
});
