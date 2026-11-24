import type { Page, Route } from '@playwright/test';

const now = () => new Date().toISOString();
const hourAgo = () => new Date(Date.now() - 3_600_000).toISOString();

const png = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
  'base64',
);

const main = {
  username: 'main',
  enabled: true,
  inRuntime: true,
  currentConnections: 9,
  activeUniqueIps: 2,
  activeIps: ['10.0.2.15', '5.129.63.108'],
  totalOctets: 102238449,
  ips24h: 2,
  ips7d: 2,
  new24h: 2,
  maxToday: 2,
  risk: 0,
  trafficToday: 216932,
  lastSeen: now(),
  maxUniqueIps: null,
  maxTcpConns: null,
  dataQuotaBytes: null,
  quotaPeriod: null,
  rateLimitUpBps: null,
  rateLimitDownBps: null,
  expiration: null,
  sourceDeny: [],
  note: 'тестовый клиент',
  link: 'tg://proxy?server=example.com&port=8444&secret=0123456789abcdef0123456789abcdef',
  foreignIpsCurrent: 0,
  foreignIps24h: 0,
  countries24h: 1,
  countries7d: 1,
  newCountries24h: 0,
  countries: ['Россия'],
  countryCodes: ['RU'],
  trustedCountries: [],
  trustedIps: [],
  ipHistory: [
    {
      ip: '10.0.2.15',
      firstSeen: hourAgo(),
      lastSeen: now(),
      count: 42,
      internal: true,
    },
    {
      ip: '5.129.63.108',
      firstSeen: hourAgo(),
      lastSeen: now(),
      count: 30,
      country: 'RU',
      countryName: 'Россия',
      asn: 31200,
      asnOrg: 'Novotelecom Ltd',
    },
  ],
};

const chernyh = {
  ...main,
  username: 'chernyh',
  activeIps: ['203.0.113.7'],
  activeUniqueIps: 1,
  currentConnections: 3,
  risk: 2,
  ips24h: 6,
  foreignIpsCurrent: 1,
  foreignIps24h: 2,
  countries24h: 2,
  countries7d: 3,
  newCountries24h: 1,
  countries: ['Нидерланды'],
  countryCodes: ['NL'],
  trustedCountries: [],
  trustedIps: [],
  ipHistory: [],
};

export const users = [main, chernyh];

/**
 * Подменяет все запросы к /api. Возвращает журнал тел PATCH-запросов
 * к лимитам, чтобы тест мог проверить отправленные значения.
 */
export async function mockApi(page: Page): Promise<{ limitBodies: unknown[] }> {
  let loggedIn = false;
  const limitBodies: unknown[] = [];

  const json = (route: Route, body: unknown, status = 200) =>
    route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });

  await page.route('**/api/**', async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname;
    const method = route.request().method();

    if (path === '/api/meta') {
      return json(route, { version: 'v0.5.0-e2e', repo: 'https://example.com/repo' });
    }
    if (path === '/api/me') {
      return loggedIn
        ? json(route, { user: { name: 'admin', role: 'admin' }, csrf: 'csrf' })
        : json(route, { error: { code: 'unauthorized', message: 'требуется вход' } }, 401);
    }
    if (path === '/api/login' && method === 'POST') {
      loggedIn = true;
      return json(route, { user: { name: 'admin', role: 'admin' }, csrf: 'csrf' });
    }
    if (path === '/api/logout') {
      loggedIn = false;
      return json(route, { ok: true });
    }
    if (!loggedIn) {
      return json(route, { error: { code: 'unauthorized', message: 'требуется вход' } }, 401);
    }
    if (path === '/api/users' && method === 'GET') {
      return json(route, users);
    }
    const userMatch = path.match(/^\/api\/users\/([^/]+)$/);
    if (userMatch && method === 'GET') {
      const found = users.find((u) => u.username === decodeURIComponent(userMatch[1]));
      if (!found) {
        return json(route, { error: { code: 'not_found', message: 'нет пользователя' } }, 404);
      }
      return json(route, found);
    }
    if (/\/api\/users\/[^/]+\/traffic$/.test(path)) {
      return json(route, [
        { day: '2026-11-22', bytes: 1048576 },
        { day: '2026-11-23', bytes: 3145728 },
        { day: '2026-11-24', bytes: 2097152 },
      ]);
    }
    if (/\/api\/users\/[^/]+\/qr\.png$/.test(path)) {
      return route.fulfill({ status: 200, contentType: 'image/png', body: png });
    }
    if (userMatch && method === 'PATCH') {
      limitBodies.push(route.request().postDataJSON());
      return json(route, users.find((u) => u.username === userMatch[1]) ?? { ok: true });
    }
    if (/\/api\/users\/[^/]+\/trusted$/.test(path)) {
      if (method === 'PUT') {
        return json(route, route.request().postDataJSON());
      }
      return json(route, { countries: [], ips: [] });
    }
    if (path === '/api/summary') {
      return json(route, {
        users: 2,
        enabled: 1,
        riskHigh: 1,
        riskWarn: 0,
        foreignCurrent: 1,
        newCountries: 1,
        trafficToday: 217_000,
        collector: { up: true, lastSuccess: now() },
        geoip: { loaded: true, built: now() },
      });
    }
    if (/\/api\/users\/[^/]+\/timeline$/.test(path)) {
      return json(route, [
        { day: '2026-11-23', newIps: 1, newCountries: [{ code: 'NL', name: 'Нидерланды' }], maxIps: 2, bytes: 3145728 },
        { day: '2026-11-24', newIps: 1, maxIps: 2, bytes: 2097152 },
      ]);
    }
    if (path === '/api/admins') {
      return json(route, [
        {
          name: 'admin',
          role: 'admin',
          disabled: false,
          createdAt: now(),
          passwordChangedAt: now(),
          sessions: 3,
        },
        {
          name: 'watcher',
          role: 'viewer',
          disabled: false,
          createdAt: now(),
          passwordChangedAt: now(),
          sessions: 1,
        },
      ]);
    }
    if (path === '/api/account/sessions') {
      return json(route, [
        { id: 'a', userAgent: 'Safari', ip: '10.0.0.21', createdAt: now(), current: true },
      ]);
    }
    if (path === '/api/audit') {
      return json(route, [
        { at: now(), actor: 'admin', action: 'user.create', target: 'main', result: 'ok', ip: '10.0.0.21' },
      ]);
    }
    return json(route, { ok: true });
  });

  return { limitBodies };
}

/** Входит в интерфейс через форму. */
export async function login(page: Page) {
  await page.goto('/');
  await page.locator('input[autocomplete="username"]').fill('admin');
  await page.locator('input[type="password"]').fill('secret');
  await page.getByRole('button', { name: 'Войти' }).click();
  await page.getByRole('link', { name: 'main', exact: true }).waitFor();
}
