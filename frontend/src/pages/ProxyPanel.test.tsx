import { screen, waitFor } from '@testing-library/react';
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
