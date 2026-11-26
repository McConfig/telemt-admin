import { vi } from 'vitest';

type Route = unknown | ((url: URL, init?: RequestInit) => unknown);

/** Подменяет fetch: точное совпадение пути, затем самый длинный префикс. */
export function stubFetch(routes: Record<string, Route>) {
  const mock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const raw = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
    const url = new URL(raw, 'http://localhost');
    let body = routes[url.pathname];
    if (body === undefined) {
      const key = Object.keys(routes)
        .filter((item) => url.pathname.startsWith(item))
        .sort((a, b) => b.length - a.length)[0];
      if (key) body = routes[key];
    }
    if (typeof body === 'function') body = (body as (u: URL, i?: RequestInit) => unknown)(url, init);
    // Готовый Response нужен для кодов, отличных от 200 (204, 401, 409).
    if (body instanceof Response) return body;
    if (body === undefined) {
      return new Response(JSON.stringify({ error: { code: 'not_found', message: 'нет' } }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      });
    }
    return new Response(JSON.stringify(body), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  });
  vi.stubGlobal('fetch', mock);
  return mock;
}
