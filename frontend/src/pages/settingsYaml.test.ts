import { describe, expect, it } from 'vitest';
import type { SettingField, SettingsCatalog } from '../api';
import { allSettingsYAML, settingsSectionYAML, templatesYAML } from './settingsYaml';

function field(key: string, value: unknown, extra: Partial<SettingField> = {}): SettingField {
  return {
    key,
    title: key,
    description: '',
    kind: 'int',
    status: 'active',
    value,
    ...extra,
  } as SettingField;
}

const catalog: SettingsCatalog = {
  version: 1,
  sections: [
    {
      id: 'risk',
      title: 'Риск',
      description: '',
      settings: [field('risk.warn_ips', 3), field('risk.home_country', 'RU')],
    },
    {
      id: 'security',
      title: 'Безопасность',
      description: '',
      settings: [
        field('security.require_2fa', true),
        field('notifications.webhook_secret', '', { secret: true }),
      ],
    },
  ],
} as SettingsCatalog;

describe('settingsYaml', () => {
  it('выгружает секцию YAML и учитывает черновик', () => {
    const text = settingsSectionYAML(catalog, 'risk', { 'risk.warn_ips': 4 });
    expect(text).toBe('risk:\n  warn_ips: 4\n  home_country: "RU"\n');
  });

  it('не выгружает секреты', () => {
    const text = allSettingsYAML(catalog, {});
    expect(text).toContain('security:');
    expect(text).toContain('require_2fa: true');
    expect(text).not.toContain('webhook_secret');
  });

  it('выгружает шаблоны лимитов', () => {
    const text = templatesYAML({
      default: 'default',
      templates: [{ name: 'default', maxIps: 3, sourceDeny: ['203.0.113.0/24'] }],
    });
    expect(text).toContain('limitTemplates:');
    expect(text).toContain('- name: "default"');
    expect(text).toContain('maxIps: 3');
    expect(text).toContain('sourceDeny');
    expect(text).toContain('203.0.113.0/24');
    expect(text).toContain('defaultTemplate: "default"');
  });
});
