import { expect, test } from '@playwright/test';
import { login, mockApi } from './mock';

test('создание клиента', async ({ page }) => {
  await mockApi(page);
  await login(page);
  await page.goto('/users/new');

  await page.getByPlaceholder('Имя').fill('newclient');
  await page.getByRole('button', { name: 'Создать' }).click();
  await expect(page).toHaveURL(/\/users\/newclient$/);
  await expect(page.locator('.page-head').getByText('newclient')).toBeVisible();
});

test('переименование клиента', async ({ page }) => {
  await mockApi(page);
  await login(page);
  await page.goto('/users/main');

  await page.getByRole('button', { name: 'Переименовать' }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Новое имя').fill('main2');
  await dialog.getByRole('button', { name: 'Переименовать' }).click();
  await expect(page).toHaveURL(/\/users\/main2$/);
  await expect(page.locator('.page-head').getByText('main2')).toBeVisible();
});

test('перевыпуск секрета', async ({ page }) => {
  await mockApi(page);
  await login(page);
  await page.goto('/users/main');

  await page.getByRole('button', { name: 'Перевыпустить секрет' }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Имя').fill('main');
  await dialog.getByRole('button', { name: 'Перевыпустить' }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByText('не удалось')).toHaveCount(0);
});

test('удаление клиента', async ({ page }) => {
  await mockApi(page);
  await login(page);
  await page.goto('/users/chernyh');

  await page.getByRole('button', { name: 'Удалить' }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Имя').fill('chernyh');
  await dialog.getByRole('button', { name: 'Удалить' }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole('link', { name: 'chernyh', exact: true })).toHaveCount(0);
});
