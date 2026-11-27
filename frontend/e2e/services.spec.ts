import { expect, test } from '@playwright/test';
import { login, mockApi } from './mock';

test('сервисный аккаунт: выпуск токена, права и отзыв', async ({ page }) => {
  await mockApi(page);
  await login(page);
  page.on('dialog', (dialog) => void dialog.accept());

  await page.getByRole('button', { name: 'Сервисные аккаунты' }).click();
  await expect(page.getByText('Сервисные аккаунты', { exact: true }).first()).toBeVisible();

  await page.getByRole('button', { name: 'Создать сервис' }).click();
  await page.getByPlaceholder('Имя').fill('monitoring');
  await page.getByPlaceholder('Описание').fill('Prometheus');
  await page.getByRole('dialog').getByRole('button', { name: 'Создать', exact: true }).click();
  await expect(page.getByText('monitoring')).toBeVisible();

  await page.getByRole('button', { name: 'Токены monitoring' }).click();
  await page.getByRole('button', { name: 'Выпустить токен' }).click();
  await page.getByPlaceholder('Метка (необязательно)').fill('ci');
  await page.getByRole('dialog').getByRole('button', { name: 'Выпустить', exact: true }).click();

  const token = (await page.locator('code').first().textContent())?.trim() ?? '';
  expect(token).toMatch(/^tgadm_/);
  await expect(page.getByText(/показывается один раз/)).toBeVisible();

  const readUsers = await page.evaluate(async (value) => {
    const res = await fetch('/api/users', {
      headers: { Authorization: `Bearer ${value}` },
    });
    return res.status;
  }, token);
  expect(readUsers).toBe(200);

  const createDenied = await page.evaluate(async (value) => {
    const res = await fetch('/api/users', {
      method: 'POST',
      headers: { Authorization: `Bearer ${value}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: 'from-token' }),
    });
    return res.status;
  }, token);
  expect(createDenied).toBe(403);

  await page.getByRole('button', { name: 'Готово' }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Закрыть' }).last().click();
  await page.getByRole('button', { name: 'Изменить monitoring' }).click();
  await page.getByRole('combobox', { name: 'Права' }).click();
  await page.getByRole('option', { name: 'Клиенты — управление клиентами' }).click();
  await page.getByRole('button', { name: 'Сохранить' }).click();
  await expect(page.locator('table').getByText('Клиенты', { exact: true })).toBeVisible();

  const createAllowed = await page.evaluate(async (value) => {
    const res = await fetch('/api/users', {
      method: 'POST',
      headers: { Authorization: `Bearer ${value}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: 'from-token' }),
    });
    return res.status;
  }, token);
  expect(createAllowed).toBe(201);

  await page.getByRole('button', { name: 'Токены monitoring' }).click();
  await page.getByRole('button', { name: 'Отозвать ci' }).click();
  await expect(page.getByText('Токенов пока нет.')).toBeVisible();

  const revoked = await page.evaluate(async (value) => {
    const res = await fetch('/api/users', {
      headers: { Authorization: `Bearer ${value}` },
    });
    return res.status;
  }, token);
  expect(revoked).toBe(401);
});
