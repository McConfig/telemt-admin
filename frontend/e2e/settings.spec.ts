import { expect, test } from '@playwright/test';
import { login, mockApi } from './mock';

test('настройки: каталог, planned и сохранение', async ({ page }) => {
  const { settingsBodies } = await mockApi(page);
  await login(page);
  await page.getByRole('link', { name: 'Настройки' }).click();

  await expect(page.getByText('Настройки', { exact: true }).last()).toBeVisible();
  await page.getByRole('button', { name: 'Безопасность' }).click();
  await expect(page.getByText('1.5.0', { exact: true })).toBeVisible();
  await expect(page.getByText('Политика для всех администраторов')).toBeVisible();
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
