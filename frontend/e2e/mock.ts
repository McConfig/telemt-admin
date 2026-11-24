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
export async function mockApi(
  page: Page,
  options: { totp?: boolean } = {},
): Promise<{ limitBodies: unknown[] }> {
  let loggedIn = false;
  const limitBodies: unknown[] = [];
  // Свежая копия на каждый тест: создание, переименование и удаление
  // не должны протекать в соседние тесты.
  const userList = users.map((u) => ({
    ...u,
    ipHistory: [...u.ipHistory],
    trustedCountries: [...u.trustedCountries],
    trustedIps: [...u.trustedIps],
  }));

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
        ? json(route, { user: { name: 'mcconfig', role: 'admin' }, csrf: 'csrf' })
        : json(route, { error: { code: 'unauthorized', message: 'требуется вход' } }, 401);
    }
    if (path === '/api/login' && method === 'POST') {
      if (options.totp) return json(route, { totpRequired: true });
      loggedIn = true;
      return json(route, { user: { name: 'mcconfig', role: 'admin' }, csrf: 'csrf' });
    }
    if (path === '/api/login/totp' && method === 'POST') {
      loggedIn = true;
      return json(route, { user: { name: 'mcconfig', role: 'admin' }, csrf: 'csrf' });
    }
    if (path === '/api/logout') {
      loggedIn = false;
      return json(route, { ok: true });
    }
    if (!loggedIn) {
      return json(route, { error: { code: 'unauthorized', message: 'требуется вход' } }, 401);
    }
    if (path === '/api/users' && method === 'GET') {
      return json(route, userList);
    }
    if (path === '/api/users' && method === 'POST') {
      const body = route.request().postDataJSON() as { name: string };
      const created = {
        ...main,
        username: body.name,
        enabled: true,
        activeIps: [],
        activeUniqueIps: 0,
        currentConnections: 0,
        totalOctets: 0,
        ips24h: 0,
        ips7d: 0,
        new24h: 0,
        maxToday: 0,
        trafficToday: 0,
        risk: 0,
        countries: [],
        countryCodes: [],
        ipHistory: [],
        trustedCountries: [],
        trustedIps: [],
      };
      userList.push(created);
      return json(route, created);
    }
    const userMatch = path.match(/^\/api\/users\/([^/]+)$/);
    if (userMatch && method === 'GET') {
      const found = userList.find((u) => u.username === decodeURIComponent(userMatch[1]));
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
      return json(route, userList.find((u) => u.username === userMatch[1]) ?? { ok: true });
    }
    if (userMatch && method === 'DELETE') {
      const index = userList.findIndex((u) => u.username === userMatch[1]);
      if (index >= 0) userList.splice(index, 1);
      return route.fulfill({ status: 204, body: '' });
    }
    const renameMatch = path.match(/^\/api\/users\/([^/]+)\/rename$/);
    if (renameMatch && method === 'POST') {
      const body = route.request().postDataJSON() as { newName: string };
      const found = userList.find((u) => u.username === renameMatch[1]);
      if (found) found.username = body.newName;
      return json(route, { username: body.newName });
    }
    const rotateMatch = path.match(/^\/api\/users\/([^/]+)\/rotate$/);
    if (rotateMatch && method === 'POST') {
      const found = userList.find((u) => u.username === rotateMatch[1]);
      return json(route, found ?? { ok: true });
    }
    if (path === '/api/account/totp') {
      return json(route, { available: true, enabled: false, recoveryLeft: 0 });
    }
    if (path === '/api/account/totp/setup') {
      return json(route, {
        secret: 'JBSWY3DPEHPK3PXP',
        url: 'otpauth://totp/TeleMT%20Admin:mcconfig?secret=JBSWY3DPEHPK3PXP',
        qr: 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
      });
    }
    if (path === '/api/account/totp/enable') {
      return json(route, {
        recoveryCodes: ['AAAA-BBBB', 'CCCC-DDDD', 'EEEE-FFFF', 'GGGG-HHHH', 'IIII-JJJJ', 'KKKK-LLLL', 'MMMM-NNNN', 'OOOO-PPPP'],
      });
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
          totpEnabled: true,
        },
        {
          name: 'watcher',
          role: 'viewer',
          disabled: false,
          createdAt: now(),
          passwordChangedAt: now(),
          sessions: 1,
          totpEnabled: false,
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
