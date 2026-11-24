export type Role = 'admin' | 'viewer';

export interface Meta {
  version: string;
  repo: string;
}

export interface Me {
  user: { name: string; role: Role };
  csrf: string;
}

export interface AdminUser {
  name: string;
  role: Role;
  disabled: boolean;
  createdAt: string;
  passwordChangedAt: string;
  sessions: number;
}

export interface IPHistoryItem {
  ip: string;
  firstSeen: string;
  lastSeen: string;
  count: number;
  country?: string;
  countryName?: string;
  asn?: number;
  asnOrg?: string;
  internal?: boolean;
}

export interface ProxyUser {
  username: string;
  enabled: boolean;
  inRuntime: boolean;
  currentConnections: number;
  activeUniqueIps: number;
  activeIps: string[];
  totalOctets: number;
  maxUniqueIps: number | null;
  maxTcpConns: number | null;
  ips24h: number;
  ips7d: number;
  new24h: number;
  maxToday: number;
  trafficToday: number;
  risk: number;
  lastSeen?: string;
  dataQuotaBytes: number | null;
  quotaPeriod: string | null;
  rateLimitUpBps: number | null;
  rateLimitDownBps: number | null;
  expiration: string | null;
  sourceDeny: string[];
  note?: string;
  ipHistory?: IPHistoryItem[];
  link?: string;
  foreignIpsCurrent: number;
  foreignIps24h: number;
  countries24h: number;
  countries7d: number;
  newCountries24h: number;
  countries?: string[];
  countryCodes?: string[];
  trustedCountries: string[];
  trustedIps: string[];
}

export interface Trusted {
  countries: string[];
  ips: string[];
}

export interface Summary {
  users: number;
  enabled: number;
  riskHigh: number;
  riskWarn: number;
  foreignCurrent: number;
  newCountries: number;
  trafficToday: number;
  collector: { up: boolean; lastSuccess?: string; lastError?: string };
  geoip: { loaded: boolean; built?: string };
}

export interface TimelineDay {
  day: string;
  newIps: number;
  newCountries?: { code: string; name: string }[];
  maxIps: number;
  bytes: number;
}

export interface TrafficDay {
  day: string;
  bytes: number;
}

export interface AuditItem {
  at: string;
  actor: string;
  action: string;
  target: string;
  result: string;
  ip: string;
}

export interface SessionItem {
  id: string;
  userAgent: string;
  ip: string;
  createdAt: string;
  current: boolean;
}

export class ApiError extends Error {
  code: string;
  status: number;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

let csrf = '';

export function setCSRF(value: string) {
  csrf = value;
}

function emitUnauthorized() {
  window.dispatchEvent(new CustomEvent('tgadmin:unauthorized'));
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (method !== 'GET' && csrf) headers['X-CSRF-Token'] = csrf;
  const res = await fetch(path, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
    credentials: 'same-origin',
  });
  if (res.status === 401) {
    emitUnauthorized();
  }
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  let data: unknown = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = null;
    }
  }
  if (!res.ok) {
    const err = (data as { error?: { code?: string; message?: string } } | null)?.error;
    throw new ApiError(res.status, err?.code ?? 'http', err?.message ?? `HTTP ${res.status}`);
  }
  return data as T;
}

export const api = {
  meta: () => request<Meta>('GET', '/api/meta'),
  async me(): Promise<Me> {
    const me = await request<Me>('GET', '/api/me');
    setCSRF(me.csrf);
    return me;
  },
  async login(username: string, password: string): Promise<Me> {
    const me = await request<Me>('POST', '/api/login', { username, password });
    setCSRF(me.csrf);
    return me;
  },
  logout: () => request<void>('POST', '/api/logout'),
  users: () => request<ProxyUser[]>('GET', '/api/users'),
  user: (name: string) => request<ProxyUser>('GET', `/api/users/${encodeURIComponent(name)}`),
  create: (body: { name: string; secret?: string; maxUniqueIps?: number; maxTcpConns?: number }) =>
    request<ProxyUser>('POST', '/api/users', body),
  updateLimits: (name: string, body: Record<string, unknown>) =>
    request<ProxyUser>('PATCH', `/api/users/${encodeURIComponent(name)}`, body),
  rotate: (name: string) =>
    request<ProxyUser>('POST', `/api/users/${encodeURIComponent(name)}/rotate`, { confirm: name }),
  enable: (name: string) => request<ProxyUser>('POST', `/api/users/${encodeURIComponent(name)}/enable`),
  disable: (name: string) => request<ProxyUser>('POST', `/api/users/${encodeURIComponent(name)}/disable`),
  rename: (name: string, newName: string) =>
    request<{ username: string }>('POST', `/api/users/${encodeURIComponent(name)}/rename`, { newName }),
  remove: (name: string) =>
    request<void>('DELETE', `/api/users/${encodeURIComponent(name)}`, { confirm: name }),
  audit: (params: {
    q?: string;
    target?: string;
    action?: string;
    from?: string;
    to?: string;
    limit?: number;
    offset?: number;
  } = {}) => {
    const query = new URLSearchParams();
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== '' && value !== 0) query.set(key, String(value));
    }
    const suffix = query.toString();
    return request<AuditItem[]>('GET', `/api/audit${suffix ? `?${suffix}` : ''}`);
  },
  sessions: () => request<SessionItem[]>('GET', '/api/account/sessions'),
  changePassword: (current: string, next: string) =>
    request<void>('POST', '/api/account/password', { current, new: next }),
  revoke: (id: string) => request<void>('POST', `/api/account/sessions/${id}/revoke`),
  qrUrl: (name: string) => `/api/users/${encodeURIComponent(name)}/qr.png`,
  resetQuota: (name: string) => request<ProxyUser>('POST', `/api/users/${encodeURIComponent(name)}/reset-quota`),
  setNote: (name: string, note: string) =>
    request<{ note: string }>('PUT', `/api/users/${encodeURIComponent(name)}/note`, { note }),
  deleteHistory: (name: string) => request<void>('DELETE', `/api/users/${encodeURIComponent(name)}/history`),
  historyCSVUrl: (name: string) => `/api/users/${encodeURIComponent(name)}/history.csv`,
  summary: () => request<Summary>('GET', '/api/summary'),
  trusted: (name: string) =>
    request<Trusted>('GET', `/api/users/${encodeURIComponent(name)}/trusted`),
  setTrusted: (name: string, body: Trusted) =>
    request<Trusted>('PUT', `/api/users/${encodeURIComponent(name)}/trusted`, body),
  userTimeline: (name: string, days = 14) =>
    request<TimelineDay[]>(
      'GET',
      `/api/users/${encodeURIComponent(name)}/timeline?days=${days}`,
    ),
  userTraffic: (name: string, days = 30) =>
    request<TrafficDay[]>(
      'GET',
      `/api/users/${encodeURIComponent(name)}/traffic?days=${days}`,
    ),
  auditCSVUrl: () => '/api/audit.csv',
  admins: () => request<AdminUser[]>('GET', '/api/admins'),
  createAdmin: (body: { name: string; role: Role; password?: string }) =>
    request<{ user: AdminUser; password?: string }>('POST', '/api/admins', body),
  patchAdmin: (name: string, body: { role?: Role; disabled?: boolean }) =>
    request<AdminUser>('PATCH', `/api/admins/${encodeURIComponent(name)}`, body),
  resetAdminPassword: (name: string, password?: string) =>
    request<{ password?: string }>('POST', `/api/admins/${encodeURIComponent(name)}/password`, { password }),
  deleteAdmin: (name: string) => request<void>('DELETE', `/api/admins/${encodeURIComponent(name)}`),
  revokeAdminSessions: (name: string) =>
    request<void>('POST', `/api/admins/${encodeURIComponent(name)}/sessions/revoke`),
};
