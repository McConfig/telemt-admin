import { expect, test } from '@playwright/test';
import { login, mockApi } from './mock';

test('вход и список клиентов', async ({ page }) => {
  await mockApi(page);
  await page.goto('/');
  await expect(page.getByText('Вход в TeleMT Admin')).toBeVisible();

  await login(page);
  await expect(page.getByText('Пользователи telemt')).toBeVisible();
  await expect(page.locator('.topbar-user').getByText('mcconfig')).toBeVisible();
  await expect(page.locator('.stat-tile')).toHaveCount(6);
  await expect(page.getByRole('link', { name: 'chernyh', exact: true })).toBeVisible();
  await expect(page.getByText('Нидерланды')).toBeVisible();
});

test('фильтр «только с иностранными IP»', async ({ page }) => {
  await mockApi(page);
  await login(page);

  await page.getByText('только с иностранными IP').click();
  await expect(page.getByRole('link', { name: 'chernyh', exact: true })).toBeVisible();
  await expect(page.getByRole('link', { name: 'main', exact: true })).toBeHidden();
});

test('массовое отключение выбранных клиентов', async ({ page }) => {
  await mockApi(page);
  await login(page);

  await page.getByRole('checkbox', { name: 'Выбрать main' }).check();
  await expect(page.getByText('Выбрано: 1')).toBeVisible();
  await page.getByRole('button', { name: 'Отключить' }).click();
  await expect(page.getByText('Выбрано: 1')).toBeHidden();
});

test('фильтр «только с новыми странами»', async ({ page }) => {
  await mockApi(page);
  await login(page);

  await page.getByText('только с новыми странами').click();
  await expect(page.getByRole('link', { name: 'chernyh', exact: true })).toBeVisible();
  await expect(page.getByRole('link', { name: 'main', exact: true })).toBeHidden();
});
