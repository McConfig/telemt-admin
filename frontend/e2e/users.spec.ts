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

test('сортировка по заголовкам: возрастание, убывание, сброс', async ({ page }) => {
  await mockApi(page);
  await login(page);

  const firstRow = () => page.locator('tbody tr').first().innerText();

  await page.getByRole('button', { name: /Трафик/ }).click();
  await expect.poll(firstRow).toContain('chernyh');

  await page.getByRole('button', { name: /Трафик/ }).click();
  await expect.poll(firstRow).toContain('main');

  await page.getByRole('button', { name: /Трафик/ }).click();
  await expect.poll(firstRow).toContain('main');
  await expect(page.locator('.th-sort-active')).toHaveCount(0);
});

test('сортировка по статусу', async ({ page }) => {
  await mockApi(page);
  await login(page);
  const firstRow = () => page.locator('tbody tr').first().innerText();

  await page.getByRole('button', { name: /Статус/ }).click();
  await expect.poll(firstRow).toContain('main');
  await page.getByRole('button', { name: /Статус/ }).click();
  await expect.poll(firstRow).toContain('chernyh');
});

test('несколько сортировок одновременно', async ({ page }) => {
  await mockApi(page);
  await login(page);

  await page.getByRole('button', { name: /Риск/ }).click();
  await page.getByRole('button', { name: /Имя/ }).click();
  await expect(page.locator('.th-sort-active')).toHaveCount(2);
  await expect(page.locator('.th-sort-mark')).toHaveCount(2);
});

test('поиск по оператору', async ({ page }) => {
  await mockApi(page);
  await login(page);

  await page.getByPlaceholder('Поиск по имени, стране, оператору').fill('LeaseWeb');
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
