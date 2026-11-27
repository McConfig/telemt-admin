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
  operators: ['Novotelecom Ltd'],
  trustedCountries: [],
  trustedIps: [],
  riskThresholds: { warnIps: null, highIps: null, distinct7d: null, newCountries: null },
  riskDefaults: { warnIps: 3, highIps: 5, distinct7d: 15, newCountries: true },
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
  enabled: false,
  activeIps: ['203.0.113.7'],
  activeUniqueIps: 1,
  currentConnections: 3,
  risk: 2,
  totalOctets: 50000000,
  ips24h: 6,
  foreignIpsCurrent: 1,
  foreignIps24h: 2,
  countries24h: 2,
  countries7d: 3,
  newCountries24h: 1,
  countries: ['Нидерланды'],
  countryCodes: ['NL'],
  operators: ['LeaseWeb'],
  trustedCountries: [],
  trustedIps: [],
  ipHistory: [],
};

export const users = [main, chernyh];

const riskDay = new Date().toISOString().slice(0, 10);
const riskReport = {
  user: 'chernyh',
  level: 2,
  custom: false,
  thresholds: { warnIps: 3, highIps: 5, distinct7d: 15, newCountries: true },
  current: {
    activeIps: 1,
    trustedActive: 0,
    current: 1,
    distinct24h: 6,
    distinct7d: 8,
    maxToday: 1,
    newIps24h: 2,
    newCountries: 1,
    foreignIps24h: 1,
    countries: [{ code: 'NL', name: 'Нидерланды' }],
  },
  reasons: [
    {
      code: 'new_country',
      severity: 1,
      value: 1,
      countries: ['NL'],
      message: 'Новые страны за сутки: NL',
    },
  ],
  days: [
    {
      day: riskDay,
      distinct: 1,
      max: 1,
      newIps: 1,
      newCountries: [{ code: 'NL', name: 'Нидерланды' }],
      traffic: 50000000,
      ips: [
        {
          ip: '203.0.113.7',
          country: 'NL',
          countryName: 'Нидерланды',
          asn: 60781,
          asnOrg: 'LeaseWeb',
          dayFirst: new Date().toISOString(),
          dayLast: new Date().toISOString(),
          count: 12,
          new: true,
          trusted: false,
        },
      ],
    },
  ],
};

/**
 * Подменяет все запросы к /api. Возвращает журнал тел PATCH-запросов
 * к лимитам, чтобы тест мог проверить отправленные значения.
 */
const settingsCatalog = {
  version: 0,
  sections: [
    {
      id: 'risk',
      title: 'Риск',
      description: 'Глобальные пороги',
      settings: [
        {
          key: 'risk.warn_ips',
          title: 'Предупреждение, больше IP',
          description: 'Порог предупреждения',
          kind: 'int',
          status: 'active',
          value: 3,
          default: 3,
          source: 'env',
          min: 0,
          max: 1000,
        },
        {
          key: 'risk.high_ips',
          title: 'Высокий риск, больше IP',
          description: 'Порог высокого риска',
          kind: 'int',
          status: 'active',
          value: 5,
          default: 5,
          source: 'env',
          min: 0,
          max: 1000,
        },
        {
          key: 'risk.home_country',
          title: 'Домашняя страна',
          description: 'Код ISO',
          kind: 'string',
          status: 'active',
          value: 'RU',
          default: 'RU',
          source: 'env',
        },
      ],
    },
    {
      id: 'links',
      title: 'Ссылки',
      description: 'Режим ссылок',
      settings: [],
    },
    {
      id: 'security',
      title: 'Безопасность',
      description: 'Входы',
      settings: [
        {
          key: 'login.max_failures',
          title: 'Неудачных входов',
          description: 'Сколько попыток до блокировки',
          kind: 'int',
          status: 'active',
          value: 10,
          default: 10,
          source: 'env',
          min: 1,
          max: 1000,
        },
        {
          key: 'backup.offsite',
          title: 'Копия вне хоста',
          description: 'Копии вне хоста пока нет',
          kind: 'bool',
          status: 'planned',
          plannedRelease: '1.6.0',
          default: false,
        },
      ],
    },
    {
      id: 'backups',
      title: 'Бэкапы',
      description: 'Расписание',
      settings: [],
    },
    {
      id: 'infrastructure',
      title: 'Инфраструктура',
      description: 'Только чтение',
      settings: [
        {
          key: 'app.listen',
          title: 'Адрес приложения',
          description: 'Меняется через env',
          kind: 'string',
          status: 'readonly',
          value: ':8090',
          source: 'env',
        },
      ],
    },
  ],
};

export async function mockApi(
  page: Page,
  options: { totp?: boolean; require2fa?: boolean; require2faEnforced?: boolean } = {},
): Promise<{ limitBodies: unknown[]; settingsBodies: unknown[] }> {
  const require2fa = options.require2fa
    ? {
        enabled: true,
        enforced: Boolean(options.require2faEnforced),
        daysLeft: options.require2faEnforced ? 0 : 7,
      }
    : undefined;
  let loggedIn = false;
  const limitBodies: unknown[] = [];
  const settingsBodies: unknown[] = [];
  // Свежая копия на каждый тест: создание, переименование и удаление
  // не должны протекать в соседние тесты.
  const userList = users.map((u) => ({
    ...u,
    ipHistory: [...u.ipHistory],
    trustedCountries: [...u.trustedCountries],
    trustedIps: [...u.trustedIps],
  }));

  let serviceList: Array<Record<string, unknown>> = [];
  let tokenList: Array<Record<string, unknown>> = [];
  let tokenScope = 'read';
  const fullServiceToken = 'tgadm_0123456789abcdef_secret';
  let tokenRevoked = false;

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
        ? json(route, { user: { name: 'mcconfig', role: 'admin' }, csrf: 'csrf', require2fa })
        : json(route, { error: { code: 'unauthorized', message: 'требуется вход' } }, 401);
    }
    if (path === '/api/login' && method === 'POST') {
      if (options.totp) return json(route, { totpRequired: true });
      loggedIn = true;
      return json(route, { user: { name: 'mcconfig', role: 'admin' }, csrf: 'csrf', require2fa });
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
    const authHeader = route.request().headers()['authorization'] ?? '';
    if (authHeader.startsWith('Bearer ')) {
      const value = authHeader.slice('Bearer '.length);
      if (tokenRevoked || value !== fullServiceToken) {
        return json(route, { error: { code: 'invalid_token', message: 'неверный или истёкший токен' } }, 401);
      }
      if (path === '/api/users' && method === 'GET') {
        return json(route, userList);
      }
      if (path === '/api/users' && method === 'POST') {
        if (tokenScope !== 'write' && tokenScope !== 'admin') {
          return json(route, { error: { code: 'forbidden', message: 'нет прав' } }, 403);
        }
        const body = route.request().postDataJSON() as { name: string };
        const created = { ...main, username: body.name, activeIps: [], ipHistory: [] };
        userList.push(created);
        return json(route, created, 201);
      }
      if (path === '/api/settings' && method === 'GET') {
        return tokenScope === 'admin' ? json(route, { sections: [] }) : json(route, { error: { code: 'forbidden', message: 'нет прав' } }, 403);
      }
      return json(route, { error: { code: 'forbidden', message: 'нет прав' } }, 403);
    }
    if (path === '/api/services' && method === 'GET') {
      return json(route, serviceList);
    }
    if (path === '/api/services' && method === 'POST') {
      const body = route.request().postDataJSON() as { name: string; description: string; scope: string };
      const created = {
        name: body.name, description: body.description ?? '', scope: body.scope,
        disabled: false, tokens: 0, createdAt: now(), updatedAt: now(), createdBy: 'mcconfig',
      };
      serviceList = [...serviceList, created];
      return json(route, created, 201);
    }
    const serviceMatch = path.match(/^\/api\/services\/([^/]+)$/);
    if (serviceMatch && method === 'PATCH') {
      const body = route.request().postDataJSON() as { scope?: string; disabled?: boolean; description?: string };
      serviceList = serviceList.map((item) =>
        item.name === serviceMatch[1] ? { ...item, ...body, updatedAt: now() } : item,
      );
      if (body.scope) tokenScope = body.scope;
      return json(route, serviceList.find((item) => item.name === serviceMatch[1]));
    }
    if (serviceMatch && method === 'DELETE') {
      serviceList = serviceList.filter((item) => item.name !== serviceMatch[1]);
      tokenList = [];
      tokenRevoked = true;
      return route.fulfill({ status: 204, body: '' });
    }
    const tokenMatch = path.match(/^\/api\/services\/([^/]+)\/tokens$/);
    if (tokenMatch && method === 'GET') {
      return json(route, tokenList);
    }
    if (tokenMatch && method === 'POST') {
      const body = route.request().postDataJSON() as { label?: string };
      const issued = {
        id: '0123456789abcdef', token: fullServiceToken, label: body.label ?? '',
        prefix: 'tgadm_0123456789abcdef', createdAt: now(),
        expiresAt: '2027-02-24T10:00:00Z',
      };
      tokenList = [{ ...issued, token: undefined, createdBy: 'mcconfig' }];
      tokenRevoked = false;
      return json(route, issued, 201);
    }
    const tokenDelete = path.match(/^\/api\/services\/([^/]+)\/tokens\/([^/]+)$/);
    if (tokenDelete && method === 'DELETE') {
      tokenList = [];
      tokenRevoked = true;
      return route.fulfill({ status: 204, body: '' });
    }
    if (path === '/api/diagnostics' && method === 'GET') {
      return json(route, {
        status: 'warn',
        checkedAt: new Date().toISOString(),
        version: 'v1.5.1-e2e',
        uptime: '1h',
        telemt: { url: 'http://telemt:9091', ok: true, latencyMs: 3, version: '3.5.17' },
        collector: { up: true, lastSuccess: new Date().toISOString() },
        geoip: { loaded: true, built: new Date().toISOString(), ageDays: 2 },
        valkey: { ok: true, latencyMs: 1, keys: 100, memory: '1M' },
        webhooks: { configured: false, events: [], secretSet: false, sent: 0, failed: 0 },
        backups: {
          valkey: { saveInProgress: false, lastStatus: 'ok', keys: 100, memory: '1M' },
          host: { available: false },
        },
        security: {
          require2fa: false,
          graceDays: 7,
          daysLeft: 0,
          totpKeySet: true,
          adminsWithoutTotp: [],
        },
        warnings: ['статус хостовых бэкапов недоступен'],
      });
    }
    if (path === '/api/settings' && method === 'GET') {
      return json(route, settingsCatalog);
    }
    if (path === '/api/settings' && method === 'PUT') {
      settingsBodies.push(route.request().postDataJSON());
      return json(route, settingsCatalog);
    }
    if (path === '/api/settings/templates' && method === 'GET') {
      return json(route, { default: '', templates: [] });
    }
    if (path === '/api/settings/backups' && method === 'GET') {
      return json(route, {
        valkey: { saveInProgress: false, lastStatus: 'ok', keys: 120, memory: '1.2M' },
        host: { available: false },
      });
    }
    if (path === '/api/settings/system' && method === 'GET') {
      return json(route, {
        version: 'v1.3.0-e2e',
        uptime: '1h',
        collector: { up: true },
        geoip: { loaded: true },
        valkey: { keys: 120, memory: '1.2M' },
        counts: { uiUsers: 2, auditEvents: 10, templates: 0 },
      });
    }
    if (path === '/api/settings/history' && method === 'GET') {
      return json(route, []);
    }
    if (path === '/api/users/import' && method === 'POST') {
      const dryRun = new URL(route.request().url()).searchParams.get('dryRun') !== '0';
      return json(route, {
        dryRun,
        total: 2,
        create: 1,
        skip: 1,
        error: 0,
        rows: [
          { line: 2, name: 'newuser', status: 'create' },
          { line: 3, name: 'main', status: 'skip', message: 'клиент уже существует' },
        ],
      });
    }
    if (path === '/api/users/reset-quota' && method === 'POST') {
      return json(route, { total: 2, reset: 2, refused: 0, unknown: 0, notSent: 0, failed: [] });
    }
    if (/^\/api\/users\/[^/]+\/quota-reset$/.test(path)) {
      if (method === 'PUT') {
        return json(route, route.request().postDataJSON());
      }
      return json(route, { mode: 'inherit', enabled: true, useDays: 30, useTime: '04:00' });
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
    if (/\/api\/users\/[^/]+\/risk-report$/.test(path)) {
      return json(route, riskReport);
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
    if (path === '/api/config/blocks' && method === 'GET') {
      return json(route, [
        { id: 'settings', title: 'Системные настройки' },
        { id: 'limitTemplates', title: 'Шаблоны лимитов' },
        { id: 'proxy', title: 'Конфигурация прокси' },
        { id: 'users', title: 'Пользователи UI' },
        { id: 'services', title: 'Сервисные аккаунты' },
        { id: 'clients', title: 'Клиенты telemt' },
      ]);
    }
    if (path === '/api/config/export' && method === 'POST') {
      return route.fulfill({
        status: 200,
        contentType: 'application/yaml',
        headers: { 'Content-Disposition': 'attachment; filename="demo.tgadmin.yaml"' },
        body: 'apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata:\n  name: demo\n  secrets: none\nspec:\n  settings: {}\n',
      });
    }
    if (path === '/api/config/precheck' && method === 'POST') {
      return json(route, {
        id: 'plan-1',
        fileName: 'demo.tgadmin.yaml',
        blocks: [{ id: 'settings', title: 'Системные настройки', changes: 1, warnings: [] }],
        diff: { settings: [{ path: 'risk.warn_ips', before: 3, after: 4 }] },
        conflicts: { users: [{ name: 'mcconfig', action: 'keep', changes: ['role'] }] },
        deletions: {},
        expiresAt: '2026-11-27T15:00:00Z',
      });
    }
    if (path === '/api/config/apply' && method === 'POST') {
      return json(route, { results: [{ block: 'settings', status: 'applied', applied: 1 }] });
    }
    if (path === '/api/proxy/schema' && method === 'GET') {
      return json(route, {
        version: 1,
        fields: [
          {
            path: 'general.links.public_host',
            description: 'Public hostname for tg:// links',
            value: { type: 'string' },
            api: { config_read: true, config_write: true },
            apply: { watcher: 'supported', reload: 'supported', effect: 'links' },
          },
          {
            path: 'censorship.tls_domain',
            description: 'Cover domain for FakeTLS',
            value: { type: 'string' },
            api: { config_read: true, config_write: true },
            apply: { watcher: 'supported', reload: 'supported', effect: 'links' },
          },
        ],
      });
    }
    if (path === '/api/proxy/config' && method === 'GET') {
      return json(route, {
        revision: 'rev1',
        config: {
          general: { links: { public_host: 'demo.example.com' } },
          censorship: { tls_domain: 'demo.example.com' },
        },
      });
    }
    if (path === '/api/proxy/validate' && method === 'POST') {
      return json(route, { ok: true, changedPaths: ['general.links.public_host'] });
    }
    if (path === '/api/proxy/apply' && method === 'POST') {
      return json(route, {
        revision: 'rev2',
        changedPaths: ['general.links.public_host'],
        reloadId: 1,
        runtimeReloadRequired: true,
        processRestartRequired: false,
        deferredProcessFields: [],
      });
    }
    if (path === '/api/proxy/reload/1' && method === 'GET') {
      return json(route, { reload_id: 1, state: 'succeeded', phase: 'succeeded' });
    }
    if (path === '/api/account/webauthn') {
      return json(route, []);
    }
    if (path === '/api/account/webauthn/register/begin') {
      return json(route, {});
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

  return { limitBodies, settingsBodies };
}

/** Входит в интерфейс через форму. */
export async function login(page: Page, waitForApp = true) {
  await page.goto('/');
  await page.locator('input[autocomplete="username"]').fill('admin');
  await page.locator('input[type="password"]').fill('secret');
  await page.getByRole('button', { name: 'Войти' }).click();
  if (waitForApp) {
    await page.getByRole('link', { name: 'main', exact: true }).waitFor();
  }
}
