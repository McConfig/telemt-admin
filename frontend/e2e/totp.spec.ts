import { expect, test } from '@playwright/test';
import { login, mockApi } from './mock';

test('включение 2FA показывает QR и резервные коды', async ({ page }) => {
  await mockApi(page);
  await login(page);
  await page.goto('/account');

  await expect(page.getByText('Двухфакторная аутентификация')).toBeVisible();
  await page.getByRole('button', { name: 'Включить 2FA' }).click();
  await expect(page.getByAltText('QR для аутентификатора')).toBeVisible();
  await expect(page.getByText('JBSWY3DPEHPK3PXP')).toBeVisible();

  await page.getByPlaceholder('Код из приложения').fill('123456');
  await page.getByRole('button', { name: 'Подтвердить' }).click();
  await expect(page.getByText('AAAA-BBBB')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Скачать' })).toBeVisible();
});

test('вход со вторым фактором', async ({ page }) => {
  await mockApi(page, { totp: true });

  await page.goto('/');
  await page.locator('input[autocomplete="username"]').fill('mcconfig');
  await page.locator('input[type="password"]').fill('secret');
  await page.getByRole('button', { name: 'Войти' }).click();
  await expect(page.getByText('Второй фактор')).toBeVisible();

  await page.getByPlaceholder('000000').fill('123456');
  await page.getByRole('button', { name: 'Подтвердить' }).click();
  await expect(page.getByText('Пользователи telemt')).toBeVisible();
  await expect(page.locator('.topbar-user').getByText('mcconfig')).toBeVisible();
});
