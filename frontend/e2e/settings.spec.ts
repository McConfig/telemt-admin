import { expect, test } from '@playwright/test';
import { login, mockApi } from './mock';

test('настройки: каталог, planned и сохранение', async ({ page }) => {
  const { settingsBodies } = await mockApi(page);
  await login(page);
  await page.getByRole('link', { name: 'Настройки' }).click();

  await expect(page.getByText('Настройки', { exact: true }).last()).toBeVisible();
  await page.getByRole('button', { name: 'Безопасность' }).click();
  await expect(page.getByText('1.6.0', { exact: true })).toBeVisible();
  await expect(page.getByText('Копии вне хоста пока нет')).toBeVisible();
  await page.getByRole('button', { name: 'Бэкапы' }).click();
  await expect(page.getByRole('button', { name: 'Снимок сейчас' })).toBeVisible();
  await page.getByRole('button', { name: 'Риск' }).click();

  const warn = page.getByPlaceholder('по умолчанию: 3');
  await warn.fill('4');
  await page.getByRole('button', { name: 'Сохранить настройки' }).click();
  await expect.poll(() => settingsBodies.length).toBe(1);
  expect(settingsBodies[0]).toMatchObject({
    version: 0,
    values: { 'risk.warn_ips': 4 },
  });
});

test('2FA: предупреждение и ограничение доступа', async ({ page }) => {
  await mockApi(page, { require2fa: true });
  await login(page);
  await expect(page.getByText(/2FA не включена/)).toBeVisible();
});

test('2FA: после срока показывается экран настройки', async ({ page }) => {
  await mockApi(page, { require2fa: true, require2faEnforced: true });
  await login(page, false);
  await expect(page.getByText('Требуется двухфакторная аутентификация')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Выйти' })).toBeVisible();
});
