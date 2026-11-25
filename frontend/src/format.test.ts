import { describe, expect, it } from 'vitest';
import { formatBytes, formatTime } from './format';

describe('formatBytes', () => {
  it('форматирует байты, килобайты, мегабайты и терабайты', () => {
    expect(formatBytes(0)).toBe('0 Б');
    expect(formatBytes(1023)).toBe('1023 Б');
    expect(formatBytes(1024)).toBe('1.0 КиБ');
    expect(formatBytes(1536)).toBe('1.5 КиБ');
    expect(formatBytes(1024 ** 2)).toBe('1.0 МиБ');
    expect(formatBytes(1024 ** 4)).toBe('1.0 ТиБ');
    expect(formatBytes(1024 ** 5)).toBe('1024.0 ТиБ');
  });
});

describe('formatTime', () => {
  it('форматирует корректную дату', () => {
    expect(formatTime('2026-11-25T10:00:00Z')).not.toBe('2026-11-25T10:00:00Z');
  });
  it('возвращает исходную строку для некорректной даты', () => {
    expect(formatTime('не дата')).toBe('не дата');
  });
});
