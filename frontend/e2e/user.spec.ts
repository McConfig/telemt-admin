import { expect, test } from '@playwright/test';
import { login, mockApi } from './mock';

test('карточка клиента: плитки, страны, иконки действий', async ({ page }) => {
  await mockApi(page);
  await login(page);
  await page.goto('/users/main');

  await expect(page.locator('.page-head').getByText('main', { exact: true })).toBeVisible();
  await expect(page.locator('.stat-tile')).toHaveCount(13);
  await expect(page.locator('.traffic-chart .traffic-col')).toHaveCount(3);
  await expect(page.getByText('Трафик по дням')).toBeVisible();

  await page.getByRole('button', { name: 'Устройства' }).click();
  await expect(page.getByRole('columnheader', { name: 'Страна' })).toBeVisible();
  await expect(page.getByRole('columnheader', { name: 'Оператор' })).toBeVisible();
  await expect(page.getByText('Novotelecom Ltd')).toBeVisible();
  await expect(page.locator('.timeline-row')).toHaveCount(2);

  const trusted = page.locator('.form-section, .stack').filter({ hasText: 'Доверенные страны' }).first();
  await trusted.getByPlaceholder('например, TR, GE').fill('TR');
  await page.getByRole('button', { name: 'Сохранить доверенные' }).click();
  await expect(page.getByText('сохранено')).toBeVisible();

  const actions = page.locator('.page-head .icon-actions button');
  await expect(actions).toHaveCount(4);
  await expect(page.getByRole('button', { name: 'Перевыпустить секрет' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Экспорт истории в CSV' })).toBeVisible();
});

test('лимиты: сохранение отправляет введённые значения', async ({ page }) => {
  const { limitBodies } = await mockApi(page);
  await login(page);
  await page.goto('/users/main');

  await page.getByRole('button', { name: 'Лимиты' }).click();
  const firstSection = page.locator('.form-section').first();
  await firstSection.getByRole('textbox').first().fill('2');
  await page.getByRole('button', { name: 'Сохранить', exact: true }).click();

  await expect.poll(() => limitBodies.length).toBe(1);
  expect(limitBodies[0]).toMatchObject({ maxUniqueIps: 2 });
});

test('пользователи UI: действия иконками', async ({ page }) => {
  await mockApi(page);
  await login(page);
  await page.goto('/admins');

  await expect(page.locator('.icon-actions button')).toHaveCount(8);
  await expect(page.getByRole('button', { name: 'Сменить пароль' })).toHaveCount(2);
  await expect(page.getByRole('button', { name: 'Удалить' })).toHaveCount(2);
});

test('мобильная карточка без горизонтальной прокрутки', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockApi(page);
  await login(page);
  await page.goto('/users/main');

  await expect(page.locator('.page-head .icon-actions button')).toHaveCount(4);
  await expect(page.locator('.stat-tile')).toHaveCount(13);
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  );
  expect(overflow).toBeLessThanOrEqual(1);
});
