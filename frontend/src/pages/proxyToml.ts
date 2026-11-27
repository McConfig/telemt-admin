type JsonObject = Record<string, unknown>;

function isObject(value: unknown): value is JsonObject {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function isTableArray(value: unknown): value is JsonObject[] {
  return Array.isArray(value) && value.length > 0 && value.every(isObject);
}

function tomlKey(key: string): string {
  return /^[A-Za-z0-9_-]+$/.test(key) ? key : JSON.stringify(key);
}

function scalar(value: unknown): string | null {
  if (value === null || value === undefined) return null;
  if (typeof value === 'string') return JSON.stringify(value);
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  if (Array.isArray(value)) {
    const parts = value.map(scalar);
    if (parts.some((part) => part === null)) return null;
    return `[${parts.join(', ')}]`;
  }
  return null;
}

function emitTable(path: string, value: JsonObject, lines: string[], arrayTable = false) {
  const plain: Array<[string, string]> = [];
  const nested: Array<[string, JsonObject]> = [];
  const arrays: Array<[string, JsonObject[]]> = [];

  for (const [key, child] of Object.entries(value)) {
    if (isObject(child)) {
      nested.push([key, child]);
      continue;
    }
    if (isTableArray(child)) {
      arrays.push([key, child]);
      continue;
    }
    const text = scalar(child);
    if (text !== null) plain.push([key, text]);
  }

  lines.push(arrayTable ? `[[${path}]]` : `[${path}]`);
  for (const [key, text] of plain) {
    lines.push(`${tomlKey(key)} = ${text}`);
  }
  for (const [key, child] of nested) {
    emitTable(`${path}.${tomlKey(key)}`, child, lines);
  }
  for (const [key, items] of arrays) {
    for (const item of items) {
      emitTable(`${path}.${tomlKey(key)}`, item, lines, true);
    }
  }
}

export function toTOML(section: string, value: unknown): string {
  const lines: string[] = [];
  if (isTableArray(value)) {
    for (const item of value) emitTable(tomlKey(section), item, lines, true);
  } else if (isObject(value)) {
    emitTable(tomlKey(section), value, lines);
  } else {
    const text = scalar(value);
    lines.push(`${tomlKey(section)} = ${text ?? '""'}`);
  }
  return `${lines.join('\n')}\n`;
}

export function configToTOML(config: Record<string, unknown>): string {
  const parts = Object.entries(config).map(([section, value]) => toTOML(section, value));
  return parts.join('\n');
}
