import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api, ApiError, setCSRF } from './api';

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('api client', () => {
  beforeEach(() => {
    setCSRF('');
    vi.restoreAllMocks();
  });

  it('adds the CSRF header to unsafe requests', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ username: 'x' }));
    vi.stubGlobal('fetch', fetchMock);
    setCSRF('csrf-token');
    await api.create({ name: 'x' });
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect((init.headers as Record<string, string>)['X-CSRF-Token']).toBe('csrf-token');
  });

  it('does not add the CSRF header to GET', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse([]));
    vi.stubGlobal('fetch', fetchMock);
    setCSRF('csrf-token');
    await api.users();
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect((init.headers as Record<string, string>)['X-CSRF-Token']).toBeUndefined();
  });

  it('turns an error envelope into ApiError', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(jsonResponse({ error: { code: 'last_user', message: 'нельзя' } }, 409)),
    );
    await expect(api.remove('main')).rejects.toMatchObject({
      status: 409,
      code: 'last_user',
      message: 'нельзя',
    } satisfies Partial<ApiError>);
  });
});

describe('api client — граничные ответы', () => {
  it('возвращает undefined для 204', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal('fetch', fetchMock);
    await expect(api.logout()).resolves.toBeUndefined();
  });

  it('сообщает о неавторизованном запросе', async () => {
    const listener = vi.fn();
    window.addEventListener('tgadmin:unauthorized', listener);
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: { code: 'unauthorized', message: 'нет входа' } }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        }),
      ),
    );
    await expect(api.users()).rejects.toMatchObject({ status: 401, message: 'нет входа' });
    expect(listener).toHaveBeenCalledTimes(1);
    window.removeEventListener('tgadmin:unauthorized', listener);
  });

  it('не падает на ответе без JSON', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('не json', { status: 200 })));
    await expect(api.deleteHistory('main')).resolves.toBeNull();
  });

  it('использует HTTP-статус, если конверта ошибки нет', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('', { status: 502 })));
    await expect(api.diagnostics()).rejects.toMatchObject({
      status: 502,
      code: 'http',
      message: 'HTTP 502',
    });
  });

  it('поддерживает вход со вторым фактором', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ totpRequired: true }))
      .mockResolvedValueOnce(jsonResponse({ user: { name: 'admin', role: 'admin' }, csrf: 'csrf-2' }));
    vi.stubGlobal('fetch', fetchMock);
    await expect(api.login('admin', 'secret')).resolves.toEqual({ totpRequired: true });
    await expect(api.loginTotp('123456')).resolves.toMatchObject({
      user: { name: 'admin', role: 'admin' },
    });
    const init = fetchMock.mock.calls[1][1] as RequestInit;
    expect((init.headers as Record<string, string>)['X-CSRF-Token']).toBeUndefined();
  });
});
