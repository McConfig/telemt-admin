import { describe, expect, it } from 'vitest';
import { normalizeTheme, THEME_OPTIONS } from './theme';

describe('theme', () => {
  it('знает все пять режимов', () => {
    expect(THEME_OPTIONS.map((item) => item.value)).toEqual([
      'system',
      'light',
      'dark',
      'light-hc',
      'dark-hc',
    ]);
  });

  it('нормализует сохранённое значение', () => {
    expect(normalizeTheme('dark-hc')).toBe('dark-hc');
    expect(normalizeTheme('broken')).toBe('system');
    expect(normalizeTheme(null)).toBe('system');
  });
});
