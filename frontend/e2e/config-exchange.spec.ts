import { expect, test } from '@playwright/test';
import { login, mockApi } from './mock';

test('экспортирует YAML из настроек', async ({ page }) => {
  await mockApi(page);
  await login(page);
  await page.goto('/settings');
  await page.getByRole('button', { name: /^Бэкапы/ }).click();
  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('button', { name: 'Скачать YAML' }).click(),
  ]);
  expect(download.suggestedFilename()).toContain('.tgadmin.yaml');
});

test('проверяет и применяет конфигурацию', async ({ page }) => {
  await mockApi(page);
  await login(page);
  await page.goto('/settings');
  await page.getByRole('button', { name: /^Бэкапы/ }).click();
  await page.getByLabel('Текст YAML').fill(
    'apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata:\n  name: demo\n  secrets: none\nspec:\n  settings:\n    risk:\n      warn_ips: 4\n',
  );
  await page.getByRole('button', { name: 'Проверить' }).click();
  await expect(page.getByText('risk.warn_ips: 3 → 4')).toBeVisible();
  await page.getByRole('button', { name: 'переписать mcconfig' }).click();
  await page.getByRole('button', { name: 'Применить выбранное' }).click();
  await expect(page.getByText('Результат')).toBeVisible();
  await expect(page.getByText('applied')).toBeVisible();
});
