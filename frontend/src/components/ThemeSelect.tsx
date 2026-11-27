import { Select } from '@gravity-ui/uikit';
import { normalizeTheme, THEME_OPTIONS, type AppTheme } from '../theme';

export function ThemeSelect({
  theme,
  onThemeChange,
}: {
  theme: AppTheme;
  onThemeChange: (theme: AppTheme) => void;
}) {
  return (
    <Select
      aria-label="Тема"
      value={[theme]}
      options={THEME_OPTIONS.map((item) => ({ value: item.value, content: item.label }))}
      onUpdate={([value]) => onThemeChange(normalizeTheme(value ?? null))}
      width="max"
    />
  );
}
