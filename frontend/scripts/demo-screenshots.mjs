// Скриншоты демо-стенда для README и документации.
//
// Требуется запущенный deploy/demo/docker-compose.yml и seed.
// Запуск из каталога frontend:
//
//   npm run screenshots:demo
//
// Переменные: DEMO_BASE_URL, DEMO_USER, DEMO_PASSWORD, DEMO_SHOTS_DIR.
import { chromium } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

const baseURL = process.env.DEMO_BASE_URL ?? 'http://localhost:8090';
const user = process.env.DEMO_USER ?? 'demo';
const password = process.env.DEMO_PASSWORD ?? 'demo-admin-password';
const outDir = process.env.DEMO_SHOTS_DIR
  ?? fileURLToPath(new URL('../../docs/screenshots/', import.meta.url));

await mkdir(outDir, { recursive: true });

const browser = await chromium.launch();
const context = await browser.newContext({
  viewport: { width: 1440, height: 960 },
  deviceScaleFactor: 1,
  locale: 'ru-RU',
  timezoneId: 'Europe/Moscow',
  userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 '
    + '(KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36',
});
const page = await context.newPage();

async function shot(name) {
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(500);
  await page.screenshot({ path: `${outDir}${name}.png`, fullPage: true });
  console.log(`снимок: ${name}.png`);
}

try {
  await page.goto(`${baseURL}/`, { waitUntil: 'domcontentloaded' });
  await page.getByPlaceholder('Имя').waitFor({ timeout: 30_000 });
  await shot('login');

  await page.getByPlaceholder('Имя').fill(user);
  await page.getByPlaceholder('Пароль').fill(password);
  await page.getByRole('button', { name: 'Войти' }).click();
  await page.locator('table').waitFor({ timeout: 30_000 });

  // Убираем сессии прошлых запусков, чтобы счётчики и таблицы были чистыми.
  await page.goto(`${baseURL}/account`, { waitUntil: 'domcontentloaded' });
  await page.waitForTimeout(800);
  const revoke = page.getByRole('button', { name: 'Завершить' });
  while ((await revoke.count()) > 0) {
    await revoke.first().click();
    await page.waitForTimeout(300);
  }
  await page.goto(`${baseURL}/`, { waitUntil: 'domcontentloaded' });
  await page.locator('table').waitFor({ timeout: 30_000 });
  await shot('clients');

  const pages = [
    ['client', '/users/phone-main'],
    ['audit', '/audit'],
    ['admins', '/admins'],
    ['account', '/account'],
    ['new-client', '/users/new'],
  ];
  for (const [name, path] of pages) {
    await page.goto(`${baseURL}${path}`, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(800);
    if (name === 'audit') {
      // Показываем только демо-записи, без IP docker-шлюза от реального входа.
      await page.getByPlaceholder('Поиск по имени, цели, IP').fill('192.0.2.101');
      await page.waitForTimeout(800);
    }
    await shot(name);
  }
  await page.goto(`${baseURL}/settings`, { waitUntil: 'domcontentloaded' });
  await page.getByRole('button', { name: /^Бэкапы/ }).click();
  await page.waitForTimeout(800);
  await shot('config-export');

  await page.getByLabel('Текст YAML').fill(
    'apiVersion: tgadmin.telemt/v1\nkind: InstallationBundle\nmetadata:\n  name: demo\n  secrets: none\nspec:\n  settings:\n    risk:\n      warn_ips: 4\n',
  );
  await page.getByRole('button', { name: 'Проверить' }).click();
  await page.waitForTimeout(1200);
  await shot('config-precheck');

  await page.getByRole('button', { name: /^Прокси/ }).click();
  await page.waitForTimeout(1200);
  await shot('proxy');
} finally {
  await browser.close();
}

console.log(`готово: ${outDir}`);
