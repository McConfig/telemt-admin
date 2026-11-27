// Полный e2e WebAuthn на реальном стеке с виртуальным аутентификатором
// Chromium. Запускается только при заданном REAL_BASE_URL, поэтому обычный
// `npm run e2e` и CI остаются на моках и не требуют Docker.
//
// Подготовка и запуск:
//
//   cd deploy/demo
//   docker compose up -d
//   docker compose --profile seed run --rm seed
//   cd ../../frontend
//   REAL_BASE_URL=http://localhost:8090 npm run e2e -- e2e/webauthn.real.spec.ts
//
// Переменные: REAL_BASE_URL, REAL_USER, REAL_PASSWORD.
import { expect, test, type Page } from '@playwright/test';

const base = process.env.REAL_BASE_URL ?? '';
const adminUser = process.env.REAL_USER ?? 'demo';
const adminPassword = process.env.REAL_PASSWORD ?? 'demo-admin-password';

// Отдельный пользователь, чтобы не трогать demo и не ломать скриншоты.
const user = 'passkey-e2e';
const userPassword = 'passkey-e2e-password';

test.describe('WebAuthn на реальном стеке', () => {
  test.skip(!base, 'REAL_BASE_URL не задан');

  test('регистрация, вход и удаление passkey', async ({ page, context }) => {
    test.setTimeout(120_000);
    page.on('pageerror', (err) => console.log('PAGEERROR', err.message));
    page.on('console', (msg) => {
      // 401 до входа — ожидаемая проверка сессии на странице логина.
      if (msg.type() === 'error' && !msg.text().includes('401')) {
        console.log('CONSOLE', msg.text());
      }
    });

    const cdp = await context.newCDPSession(page);
    // Виртуальный аутентификатор: присутствие пользователя подтверждается
    // автоматически, поэтому браузерный диалог не блокирует тест.
    await (cdp as unknown as CdpSender).send('WebAuthn.enable', { enableUI: false });
    await (cdp as unknown as CdpSender).send('WebAuthn.addVirtualAuthenticator', {
      options: {
        protocol: 'ctap2',
        transport: 'internal',
        hasResidentKey: true,
        hasUserVerification: true,
        isUserVerified: true,
        automaticPresenceSimulation: true,
      },
    });

    try {
      await prepareUser(page);
      await context.clearCookies();

      await login(page, user, userPassword);
      await expect(page.getByText('Пользователи telemt')).toBeVisible({ timeout: 20_000 });

      // 1. Добавляем passkey в аккаунте.
      await page.goto(`${base}/account`);
      await page.getByRole('button', { name: 'Добавить passkey' }).click();
      const addDialog = page.getByRole('dialog');
      await addDialog.getByPlaceholder('Название, например MacBook').fill('Virtual authenticator');
      await addDialog.getByPlaceholder('Текущий пароль').fill(userPassword);
      await expect(addDialog.getByRole('button', { name: 'Добавить' })).toBeEnabled();
      await addDialog.getByRole('button', { name: 'Добавить' }).click();
      await expect(addDialog).toBeHidden();
      await expect(page.getByText('Virtual authenticator')).toBeVisible();

      // 2. После выхода вход по паролю требует второй фактор.
      await context.clearCookies();
      await login(page, user, userPassword);
      await expect(page.getByText('Второй фактор')).toBeVisible({ timeout: 20_000 });
      await page.getByRole('button', { name: 'Войти с passkey' }).click();
      await expect(page.getByText('Пользователи telemt')).toBeVisible({ timeout: 20_000 });

      // 3. Удаляем passkey с текущим паролем.
      await page.goto(`${base}/account`);
      const card = page.locator('.g-card').filter({ hasText: 'Passkeys' });
      await card.getByRole('button', { name: 'Удалить' }).click();
      const deleteDialog = page.getByRole('dialog');
      await deleteDialog.getByPlaceholder('Текущий пароль').fill(userPassword);
      await deleteDialog.getByRole('button', { name: 'Удалить' }).click();
      await expect(deleteDialog).toBeHidden();
      await expect(page.getByText('Passkeys пока нет.')).toBeVisible();

      // 4. Без второго фактора вход снова проходит сразу.
      await context.clearCookies();
      await login(page, user, userPassword);
      await expect(page.getByText('Пользователи telemt')).toBeVisible({ timeout: 20_000 });
      await expect(page.getByText('Второй фактор')).toHaveCount(0);
    } finally {
      await removeUser(page);
    }
  });
});

type CdpSender = {
  send(method: string, params?: Record<string, unknown>): Promise<Record<string, unknown>>;
};

async function login(page: Page, name: string, password: string): Promise<void> {
  await page.goto(`${base}/`);
  await page.locator('input[autocomplete="username"]').fill(name);
  await page.locator('input[type="password"]').fill(password);
  await page.getByRole('button', { name: 'Войти' }).click();
}

async function csrf(page: Page): Promise<string> {
  const me = await (await page.request.get(`${base}/api/me`)).json();
  return me.csrf as string;
}

// Создаёт отдельного администратора и сбрасывает его 2FA. Идемпотентно:
// после упавшего прогона пользователь и passkey остаются, но reset их убирает.
async function prepareUser(page: Page): Promise<void> {
  await login(page, adminUser, adminPassword);
  await expect(page.getByText('Пользователи telemt')).toBeVisible({ timeout: 20_000 });
  const token = await csrf(page);
  const headers = { 'X-CSRF-Token': token };

  const created = await page.request.post(`${base}/api/admins`, {
    headers,
    data: { name: user, role: 'admin', password: userPassword },
  });
  expect([201, 409]).toContain(created.status());

  const password = await page.request.post(`${base}/api/admins/${user}/password`, {
    headers,
    data: { password: userPassword },
  });
  expect(password.status()).toBe(200);

  const reset = await page.request.post(`${base}/api/admins/${user}/totp-reset`, { headers });
  // 400 — у пользователя не было второго фактора, это допустимо.
  expect([204, 400]).toContain(reset.status());
}

async function removeUser(page: Page): Promise<void> {
  try {
    await page.context().clearCookies();
    await login(page, adminUser, adminPassword);
    await expect(page.getByText('Пользователи telemt')).toBeVisible({ timeout: 20_000 });
    const token = await csrf(page);
    await page.request.delete(`${base}/api/admins/${user}`, {
      headers: { 'X-CSRF-Token': token },
    });
  } catch {
    // Уборка не должна скрывать исходную ошибку теста.
  }
}
