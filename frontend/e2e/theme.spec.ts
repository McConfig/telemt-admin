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

test('системная тема и сворачивание сайдбара', async ({ page }) => {
  await mockApi(page);
  await login(page);
  expect(await page.evaluate(() => localStorage.getItem('tgadmin-theme'))).toBe('system');
  await expect
    .poll(() => page.evaluate(() => document.body.className))
    .toMatch(/g-root_theme_(light|dark)/);

  await page.locator('[data-gn-aside-collapse-layer] button').first().click({ force: true });
  expect(await page.evaluate(() => localStorage.getItem('tgadmin-sidebar-compact'))).toBe(
    'true',
  );
});

test('на мобильном сайдбар свёрнут и нет горизонтального скролла', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 800 });
  await page.addInitScript(() => {
    localStorage.setItem('tgadmin-sidebar-compact', 'false');
  });
  await mockApi(page);
  await login(page);
  await expect
    .poll(() =>
      page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth),
    )
    .toBeLessThanOrEqual(1);
});
