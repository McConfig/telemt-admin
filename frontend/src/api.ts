export type Role = 'admin' | 'viewer';

export interface Me {
  user: { name: string; role: Role };
  csrf: string;
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
  link?: string;
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
  audit: () => request<AuditItem[]>('GET', '/api/audit'),
  sessions: () => request<SessionItem[]>('GET', '/api/account/sessions'),
  changePassword: (current: string, next: string) =>
    request<void>('POST', '/api/account/password', { current, new: next }),
  revoke: (id: string) => request<void>('POST', `/api/account/sessions/${id}/revoke`),
  qrUrl: (name: string) => `/api/users/${encodeURIComponent(name)}/qr.png`,
};
