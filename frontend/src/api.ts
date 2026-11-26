export type Role = 'admin' | 'viewer';

export interface Meta {
  version: string;
  repo: string;
  defaultChartDays?: number;
}

export interface Require2FAInfo {
  enabled: boolean;
  enforced: boolean;
  deadline?: string;
  daysLeft: number;
}

export interface Me {
  user: { name: string; role: Role };
  csrf: string;
  require2fa?: Require2FAInfo;
}

export interface AdminUser {
  name: string;
  role: Role;
  disabled: boolean;
  createdAt: string;
  passwordChangedAt: string;
  sessions: number;
  totpEnabled: boolean;
}

export interface TOTPStatus {
  available: boolean;
  enabled: boolean;
  createdAt?: string;
  recoveryLeft: number;
}

export interface TOTPSetup {
  secret: string;
  url: string;
  qr: string;
}

export type LoginResult = Me | { totpRequired: true };

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
  operators?: string[];
  trustedCountries: string[];
  trustedIps: string[];
  riskThresholds?: RiskThresholds;
  riskDefaults?: RiskDefaults;
}

export interface RiskThresholds {
  warnIps: number | null;
  highIps: number | null;
  distinct7d: number | null;
  newCountries: boolean | null;
}

export interface RiskDefaults {
  warnIps: number;
  highIps: number;
  distinct7d: number;
  newCountries: boolean;
}

export interface RiskCountry {
  code: string;
  name: string;
}

export interface RiskReason {
  code: string;
  severity: number;
  value: number;
  threshold?: number;
  countries?: string[];
  message: string;
}

export interface RiskIP {
  ip: string;
  country?: string;
  countryName?: string;
  asn?: number;
  asnOrg?: string;
  internal?: boolean;
  firstSeen?: string;
  lastSeen?: string;
  dayFirst?: string;
  dayLast?: string;
  count: number;
  new: boolean;
  trusted: boolean;
}

export interface RiskDay {
  day: string;
  distinct: number;
  max: number;
  newIps: number;
  newCountries?: RiskCountry[];
  traffic: number;
  ips: RiskIP[];
}

export interface RiskCurrent {
  activeIps: number;
  trustedActive: number;
  current: number;
  distinct24h: number;
  distinct7d: number;
  maxToday: number;
  newIps24h: number;
  newCountries: number;
  foreignIps24h: number;
  countries?: RiskCountry[];
}

export interface RiskReport {
  user: string;
  level: number;
  custom: boolean;
  thresholds: RiskDefaults;
  current: RiskCurrent;
  reasons: RiskReason[];
  days: RiskDay[];
}

export type SettingStatus = 'active' | 'readonly' | 'planned';
export type SettingKind = 'int' | 'bool' | 'duration' | 'string' | 'enum';

export interface SettingOption {
  value: string;
  label: string;
}

export interface SettingField {
  key: string;
  title: string;
  description: string;
  kind: SettingKind;
  status: SettingStatus;
  plannedRelease?: string;
  restartRequired?: boolean;
  secret?: boolean;
  value?: unknown;
  default?: unknown;
  source?: string;
  secretSet?: boolean;
  min?: number;
  max?: number;
  minDuration?: string;
  maxDuration?: string;
  options?: SettingOption[];
}

export interface SettingsSection {
  id: string;
  title: string;
  description: string;
  settings: SettingField[];
}

export interface SettingsCatalog {
  version: number;
  sections: SettingsSection[];
}

export interface LimitTemplate {
  name: string;
  maxIps?: number;
  maxTcpConns?: number;
  dataQuotaBytes?: number;
  quotaPeriod?: string;
  rateLimitUpBps?: number;
  rateLimitDownBps?: number;
  expirationDays?: number;
  sourceDeny?: string[];
  riskWarnIps?: number;
  riskHighIps?: number;
  riskDistinct7d?: number;
  riskNewCountries?: boolean;
}

export interface TemplatesResponse {
  default: string;
  templates: LimitTemplate[];
}

export interface BackupStatus {
  valkey: {
    lastSave?: string;
    saveInProgress: boolean;
    lastStatus?: string;
    keys: number;
    memory?: string;
  };
  host: {
    available: boolean;
    lastRun?: string;
    result?: string;
    size?: number;
    keepDays?: number;
    restoreTest?: boolean;
  };
}

export interface SettingsSystem {
  version: string;
  uptime: string;
  collector?: {
    up: boolean;
    lastSuccess?: string;
    lastError?: string;
  };
  geoip?: {
    loaded: boolean;
    built?: number;
  };
  valkey: { keys: number; memory?: string };
  counts: { uiUsers: number; auditEvents: number; templates: number };
}

export interface Diagnostics {
  status: 'ok' | 'warn' | 'error' | string;
  checkedAt: string;
  version: string;
  uptime: string;
  telemt: {
    url: string;
    ok: boolean;
    latencyMs: number;
    version?: string;
    code?: string;
    error?: string;
    hint?: string;
  };
  collector: { up: boolean; lastSuccess?: string; lastError?: string };
  geoip: { loaded: boolean; built?: string; ageDays: number };
  valkey: { ok: boolean; latencyMs: number; keys: number; memory?: string; error?: string };
  webhooks: {
    configured: boolean;
    events: string[];
    secretSet: boolean;
    lastAt?: string;
    lastEvent?: string;
    lastStatus?: number;
    lastError?: string;
    sent: number;
    failed: number;
  };
  backups: BackupStatus;
  security: {
    require2fa: boolean;
    graceDays: number;
    deadline?: string;
    daysLeft: number;
    totpKeySet: boolean;
    adminsWithoutTotp: string[];
  };
  warnings: string[];
}

export interface SettingsHistoryItem {
  At: string;
  Actor: string;
  Action: string;
  Target: string;
  Result: string;
  IP: string;
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
  async login(username: string, password: string): Promise<LoginResult> {
    const result = await request<Me & { totpRequired?: boolean }>('POST', '/api/login', {
      username,
      password,
    });
    if (result.totpRequired) return { totpRequired: true };
    setCSRF(result.csrf);
    return result;
  },
  async loginTotp(code: string): Promise<Me> {
    const me = await request<Me>('POST', '/api/login/totp', { code });
    setCSRF(me.csrf);
    return me;
  },
  logout: () => request<void>('POST', '/api/logout'),
  users: () => request<ProxyUser[]>('GET', '/api/users'),
  user: (name: string) => request<ProxyUser>('GET', `/api/users/${encodeURIComponent(name)}`),
  create: (body: {
    name: string;
    secret?: string;
    maxUniqueIps?: number;
    maxTcpConns?: number;
    dataQuotaBytes?: number;
    quotaPeriod?: string;
    rateLimitUpBps?: number;
    rateLimitDownBps?: number;
    expiration?: string;
    sourceDeny?: string[];
  }) => request<ProxyUser>('POST', '/api/users', body),
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
    sort?: string;
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
  totpStatus: () => request<TOTPStatus>('GET', '/api/account/totp'),
  totpSetup: () => request<TOTPSetup>('POST', '/api/account/totp/setup', {}),
  totpEnable: (code: string) =>
    request<{ recoveryCodes: string[] }>('POST', '/api/account/totp/enable', { code }),
  totpDisable: (password: string, code: string) =>
    request<void>('POST', '/api/account/totp/disable', { password, code }),
  totpRecovery: (password: string, code: string) =>
    request<{ recoveryCodes: string[] }>('POST', '/api/account/totp/recovery', { password, code }),
  resetAdminTOTP: (name: string) =>
    request<void>('POST', `/api/admins/${encodeURIComponent(name)}/totp-reset`),
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
  riskReport: (name: string, days = 7) =>
    request<RiskReport>('GET', `/api/users/${encodeURIComponent(name)}/risk-report?days=${days}`),
  riskCSVUrl: (name: string, days = 30) =>
    `/api/users/${encodeURIComponent(name)}/risk.csv?days=${days}`,
  settings: () => request<SettingsCatalog>('GET', '/api/settings'),
  updateSettings: (body: { version: number; values: Record<string, unknown>; reset?: string[] }) =>
    request<SettingsCatalog>('PUT', '/api/settings', body),
  templates: () => request<TemplatesResponse>('GET', '/api/settings/templates'),
  saveTemplate: (body: LimitTemplate) =>
    request<LimitTemplate>('POST', '/api/settings/templates', body),
  updateTemplate: (name: string, body: LimitTemplate) =>
    request<LimitTemplate>('PUT', `/api/settings/templates/${encodeURIComponent(name)}`, body),
  deleteTemplate: (name: string) =>
    request<void>('DELETE', `/api/settings/templates/${encodeURIComponent(name)}`),
  setDefaultTemplate: (name: string) =>
    request<{ default: string }>('PUT', '/api/settings/templates/default', { name }),
  backups: () => request<BackupStatus>('GET', '/api/settings/backups'),
  backupSnapshot: () =>
    request<{ started: boolean }>('POST', '/api/settings/backups/snapshot'),
  webhookTest: () =>
    request<{ sent: boolean; status: number }>('POST', '/api/settings/webhook/test'),
  system: () => request<SettingsSystem>('GET', '/api/settings/system'),
  settingsHistory: () => request<SettingsHistoryItem[]>('GET', '/api/settings/history'),
  diagnostics: () => request<Diagnostics>('GET', '/api/diagnostics'),
  setRisk: (name: string, body: RiskThresholds) =>
    request<{ riskThresholds: RiskThresholds; riskDefaults: RiskDefaults }>(
      'PUT',
      `/api/users/${encodeURIComponent(name)}/risk`,
      body,
    ),
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
