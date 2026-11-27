import { expect, test } from '@playwright/test';
import { login, mockApi } from './mock';

test('редактор прокси проверяет и применяет изменения', async ({ page }) => {
  await mockApi(page);
  await login(page);
  await page.goto('/settings');
  await page.getByRole('button', { name: /^Прокси/ }).click();
  const input = page.getByRole('textbox', { name: 'general.links.public_host' });
  await expect(input).toHaveValue('demo.example.com');
  await input.fill('new.example.com');
  await page.getByRole('checkbox', { name: 'Подтвердить опасные изменения' }).check();
  await page.getByRole('button', { name: 'Применить' }).click();
  await expect(page.getByText(/Применено, ревизия rev2/)).toBeVisible();
  await expect(page.getByText('Перезагрузка 1: succeeded')).toBeVisible();
});

test('подсказка параметра показывается по-русски', async ({ page }) => {
  await mockApi(page);
  await login(page);
  await page.goto('/settings');
  await page.getByRole('button', { name: /^Прокси/ }).click();
  await page.getByRole('button', { name: 'Описание general.links.public_host' }).click();
  await expect(page.getByText(/Переопределение общедоступного имени/)).toBeVisible();
});
