import type { Locale } from './types';
import { proxyDescriptionsRu } from './proxy.ru';

// Приоритет: словарь выбранного языка, затем описание из схемы telemt.
// Английский словарь не нужен: схема telemt уже английская.
export function proxyDescription(
  path: string,
  schemaDescription: string | undefined,
  locale: Locale,
): string | undefined {
  if (locale === 'ru') {
    return proxyDescriptionsRu[path] ?? schemaDescription;
  }
  return schemaDescription;
}
