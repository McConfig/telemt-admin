import type { LimitTemplate, SettingField, SettingsCatalog, TemplatesResponse } from '../api';

type Json = unknown;

function scalar(value: Json): string {
  if (value === null || value === undefined) return 'null';
  if (typeof value === 'string') return JSON.stringify(value);
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  return JSON.stringify(value);
}

function yamlKey(key: string): string {
  return /^[A-Za-z0-9_.-]+$/.test(key) ? key : JSON.stringify(key);
}

function emit(value: Json, indent: number): string[] {
  const pad = ' '.repeat(indent);
  if (Array.isArray(value)) {
    if (value.length === 0) return [`${pad}[]`];
    if (value.every((item) => item === null || typeof item !== 'object')) {
      return [`${pad}[${value.map(scalar).join(', ')}]`];
    }
    const lines: string[] = [];
    for (const item of value) {
      if (item && typeof item === 'object' && !Array.isArray(item)) {
        const entries = Object.entries(item as Record<string, Json>);
        if (entries.length === 0) {
          lines.push(`${pad}- {}`);
          continue;
        }
        const [firstKey, firstValue] = entries[0];
        const firstScalar = firstValue === null || typeof firstValue !== 'object';
        if (firstScalar) {
          lines.push(`${pad}- ${yamlKey(firstKey)}: ${scalar(firstValue)}`);
          for (const [key, child] of entries.slice(1)) {
            if (child === null || typeof child !== 'object') {
              lines.push(`${pad}  ${yamlKey(key)}: ${scalar(child)}`);
            } else {
              lines.push(`${pad}  ${yamlKey(key)}:`);
              lines.push(...emit(child, indent + 4));
            }
          }
        } else {
          lines.push(`${pad}-`);
          lines.push(...emit(item, indent + 2));
        }
      } else {
        lines.push(`${pad}- ${scalar(item)}`);
      }
    }
    return lines;
  }
  if (value && typeof value === 'object') {
    const lines: string[] = [];
    for (const [key, child] of Object.entries(value as Record<string, Json>)) {
      if (child === null || typeof child !== 'object') {
        lines.push(`${pad}${yamlKey(key)}: ${scalar(child)}`);
      } else if (Array.isArray(child)) {
        if (child.length === 0 || child.every((item) => item === null || typeof item !== 'object')) {
          lines.push(`${pad}${yamlKey(key)}: [${child.map(scalar).join(', ')}]`);
        } else {
          lines.push(`${pad}${yamlKey(key)}:`);
          lines.push(...emit(child, indent + 2));
        }
      } else {
        lines.push(`${pad}${yamlKey(key)}:`);
        lines.push(...emit(child, indent + 2));
      }
    }
    return lines;
  }
  return [`${pad}${scalar(value)}`];
}

export function toYAML(value: Json): string {
  return `${emit(value, 0).join('\n')}\n`;
}

function fieldValue(field: SettingField, draft: Record<string, unknown>): unknown {
  if (field.status === 'active' && Object.prototype.hasOwnProperty.call(draft, field.key)) {
    return draft[field.key];
  }
  return field.value;
}

function sectionObject(sectionID: string, fields: SettingField[], draft: Record<string, unknown>) {
  const out: Record<string, unknown> = {};
  for (const field of fields) {
    if (field.secret) continue;
    const value = fieldValue(field, draft);
    if (value === undefined || value === null || value === '') continue;
    const key = field.key.startsWith(sectionID + '.') ? field.key.slice(sectionID.length + 1) : field.key;
    out[key] = value;
  }
  return out;
}

export function settingsSectionYAML(
  catalog: SettingsCatalog,
  sectionID: string,
  draft: Record<string, unknown>,
): string {
  const section = catalog.sections.find((item) => item.id === sectionID);
  if (!section) return '';
  return toYAML({ [sectionID]: sectionObject(section.id, section.settings, draft) });
}

export function allSettingsYAML(
  catalog: SettingsCatalog,
  draft: Record<string, unknown>,
  templates?: TemplatesResponse,
): string {
  const out: Record<string, unknown> = {};
  for (const section of catalog.sections) {
    const values = sectionObject(section.id, section.settings, draft);
    if (Object.keys(values).length > 0) out[section.id] = values;
  }
  if (templates) {
    out.limitTemplates = templates.templates.map(templateObject);
    out.defaultTemplate = templates.default || null;
  }
  return toYAML(out);
}

function templateObject(template: LimitTemplate): Record<string, unknown> {
  const out: Record<string, unknown> = { name: template.name };
  for (const [key, value] of Object.entries(template)) {
    if (key === 'name' || value === undefined) continue;
    out[key] = value;
  }
  return out;
}

export function templatesYAML(data: TemplatesResponse): string {
  return toYAML({
    limitTemplates: data.templates.map(templateObject),
    defaultTemplate: data.default || null,
  });
}
