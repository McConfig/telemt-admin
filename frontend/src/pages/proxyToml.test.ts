import { describe, expect, it } from 'vitest';
import { configToTOML, toTOML } from './proxyToml';

describe('toTOML', () => {
  it('пишет скаляры и вложенные таблицы', () => {
    const text = toTOML('general', {
      log_level: 'normal',
      links: { public_host: 'demo.example.com', public_port: 443 },
      modes: { classic: false, tls: true },
    });
    expect(text).toContain('[general]');
    expect(text).toContain('log_level = "normal"');
    expect(text).toContain('[general.links]');
    expect(text).toContain('public_host = "demo.example.com"');
    expect(text).toContain('public_port = 443');
    expect(text).toContain('[general.modes]');
    expect(text).toContain('tls = true');
  });

  it('пишет массивы и массивы таблиц', () => {
    const text = toTOML('upstreams', [{ address: '1.1.1.1:443', enabled: true }]);
    expect(text).toContain('[[upstreams]]');
    expect(text).toContain('address = "1.1.1.1:443"');
    expect(text).toContain('enabled = true');
  });

  it('экранирует ключи карты', () => {
    const text = toTOML('dc_overrides', { '203': ['1.1.1.1:443'] });
    expect(text).toContain('203 = ["1.1.1.1:443"]');
  });

  it('собирает полный документ', () => {
    const text = configToTOML({ general: { log_level: 'info' }, censorship: { mask: true } });
    expect(text).toContain('[general]');
    expect(text).toContain('[censorship]');
  });
});
