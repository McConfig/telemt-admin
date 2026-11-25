import { expect, test } from '@playwright/test';
import { login, mockApi } from './mock';

test('сортировка в пользователях UI', async ({ page }) => {
  await mockApi(page);
  await login(page);
  await page.goto('/admins');
  await page.getByRole('button', { name: /Имя/ }).first().click();
  await expect(page.locator('.th-sort-mark')).toHaveCount(1);
});

test('сортировка в сессиях аккаунта', async ({ page }) => {
  await mockApi(page);
  await login(page);
  await page.goto('/account');
  await page.getByRole('button', { name: /Устройство/ }).click();
  await expect(page.locator('.th-sort-mark')).toHaveCount(1);
});

test('сортировка в аудите', async ({ page }) => {
  await mockApi(page);
  await login(page);
  await page.goto('/audit');
  await expect(page.locator('.th-sort-mark')).toHaveCount(1);
  await page.getByRole('button', { name: /^Кто/ }).click();
  await expect(page.locator('.th-sort-mark')).toHaveCount(2);
});
