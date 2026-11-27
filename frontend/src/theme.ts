export type AppTheme = 'system' | 'light' | 'dark' | 'light-hc' | 'dark-hc';

export const THEME_OPTIONS: { value: AppTheme; label: string }[] = [
  { value: 'system', label: 'Как в системе' },
  { value: 'light', label: 'Светлая' },
  { value: 'dark', label: 'Тёмная' },
  { value: 'light-hc', label: 'Светлая контрастная' },
  { value: 'dark-hc', label: 'Тёмная контрастная' },
];

export function normalizeTheme(raw: string | null): AppTheme {
  return THEME_OPTIONS.some((item) => item.value === raw)
    ? (raw as AppTheme)
    : 'system';
}
