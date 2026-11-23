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
