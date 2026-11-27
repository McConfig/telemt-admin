import { expect, test } from '@playwright/test';
import { login, mockApi } from './mock';

test('переключает все темы Gravity UI', async ({ page }) => {
  await mockApi(page);
  await login(page);

  for (const [label, cls] of [
    ['Светлая', 'g-root_theme_light'],
    ['Тёмная', 'g-root_theme_dark'],
    ['Светлая контрастная', 'g-root_theme_light-hc'],
    ['Тёмная контрастная', 'g-root_theme_dark-hc'],
  ] as const) {
    await page.getByRole('combobox', { name: 'Тема' }).click();
    await page.getByRole('option', { name: label, exact: true }).click();
    await expect
      .poll(() => page.evaluate(() => document.body.className))
      .toContain(cls);
    expect(await page.evaluate(() => localStorage.getItem('tgadmin-theme'))).toBe(
      label === 'Светлая'
        ? 'light'
        : label === 'Тёмная'
          ? 'dark'
          : label === 'Светлая контрастная'
            ? 'light-hc'
            : 'dark-hc',
    );
  }
});
