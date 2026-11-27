# UI-аудит Gravity UI: план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) или
> superpowers:executing-plans. Шаги отмечены чекбоксами (`- [ ]`).

**Goal:** перевести интерфейс на `AsideHeader`, подключить все темы uikit и
заменить нативные формы, кнопки и preview-таблицы компонентами Gravity UI.

**Architecture:** `Provider` uikit становится единым источником темы и
русской локали. `Layout` строится на `AsideHeader`; страницы используют
общие обёртки `FormField`, `FormSelect`, `DateFormField`. Нативные
контролы в `src` исчезают, кроме скрытого input внутри `useFileInput`.

**Tech Stack:** React 19, TypeScript, Vite, Gravity UI uikit 8.2,
`@gravity-ui/navigation` 8.x, `@gravity-ui/table` 2.x,
`@gravity-ui/date-components` 5.x, `@gravity-ui/date-utils` 2.x,
Vitest, Playwright.

**Spec:** `docs/superpowers/specs/2026-11-27-ui-gravity-audit-design.md`

## Global Constraints

- Все тексты — на русском, без эмодзи.
- Версии Gravity UI берутся из установленных пакетов, не из памяти.
- Не выдумывать пропсы: при сомнении читать
  `node_modules/@gravity-ui/<pkg>/build/docs` или `.d.ts`.
- После установки зависимостей зафиксировать `package-lock.json`.
- Не менять backend, API и бизнес-логику.
- Пороги: UI 85% строк/инструкций/функций, 70% ветвей; Go-пороги не
  меняются.
- После каждого этапа: `npm run typecheck` и точечные тесты; полный
  `npm run coverage` и `npm run e2e` перед PR.
- `#fff` допустим только под QR/TOTP с комментарием о сканируемости.
- Нативные `select`, `input`, `textarea`, `button`, `table` в `src` не
  остаются, кроме скрытого input из `useFileInput` и случаев, где сам
  компонент uikit рендерит нативный элемент.

## Review Focus

1. Все 5 тем реально переключаются и сохраняются; `system` следует ОС, а
   `light-hc`/`dark-hc` не ломают контраст и QR.
2. Сайдбар не ломает права: viewer не видит админские пункты; текущий
   пункт подсвечивается на вложенных маршрутах.
3. Скрытое состояние сайдбара сохраняется, на мобильном не перекрывает
   контент и не вызывает горизонтальный скролл.
4. Gravity `Select`/`DateField` сохраняют прежние значения и отправляют
   те же строки в API, включая пустые даты и `clear*`.
5. Миграция не ломает сценарии: импорт CSV, выпуск токенов, настройки,
   аудит и мобильные карточки.

---

## Task 1: зависимости, провайдер и модель тем

**Files:**
- Modify: `frontend/package.json`, `frontend/package-lock.json`
- Create: `frontend/src/theme.ts`
- Create: `frontend/src/theme.test.ts`
- Create: `frontend/src/providers/AppProviders.tsx`

**Interfaces:**
- Produces:
  - `type AppTheme = 'system' | 'light' | 'dark' | 'light-hc' | 'dark-hc'`
  - `const THEME_OPTIONS: {value: AppTheme; label: string}[]`
  - `function normalizeTheme(raw: string | null): AppTheme`
  - `function AppProviders({theme, children}: {theme: AppTheme; children: React.ReactNode})`

- [ ] **Step 1: установить зависимости**

```bash
cd frontend
npm install @gravity-ui/navigation @bem-react/classname \
  @gravity-ui/date-components @gravity-ui/date-utils
```

Проверить, что в `node_modules` версии:
`navigation 8.x`, `date-components 5.x`, `date-utils 2.x`,
`uikit 8.x`.

- [ ] **Step 2: написать падающий тест модели тем**

```ts
import {describe, expect, it} from 'vitest';
import {normalizeTheme, THEME_OPTIONS} from './theme';

describe('theme', () => {
  it('знает все пять режимов', () => {
    expect(THEME_OPTIONS.map((item) => item.value)).toEqual([
      'system', 'light', 'dark', 'light-hc', 'dark-hc',
    ]);
  });
  it('нормализует сохранённое значение', () => {
    expect(normalizeTheme('dark-hc')).toBe('dark-hc');
    expect(normalizeTheme('broken')).toBe('system');
    expect(normalizeTheme(null)).toBe('system');
  });
});
```

- [ ] **Step 3: запустить тест и убедиться, что падает**

Run: `cd frontend && npx vitest run src/theme.test.ts`
Expected: FAIL, модуль не найден.

- [ ] **Step 4: реализовать `theme.ts` и `AppProviders`**

```ts
export type AppTheme = 'system' | 'light' | 'dark' | 'light-hc' | 'dark-hc';

export const THEME_OPTIONS: {value: AppTheme; label: string}[] = [
  {value: 'system', label: 'Как в системе'},
  {value: 'light', label: 'Светлая'},
  {value: 'dark', label: 'Тёмная'},
  {value: 'light-hc', label: 'Светлая контрастная'},
  {value: 'dark-hc', label: 'Тёмная контрастная'},
];

export function normalizeTheme(raw: string | null): AppTheme {
  return THEME_OPTIONS.some((item) => item.value === raw)
    ? (raw as AppTheme)
    : 'system';
}
```

```tsx
import {Provider} from '@gravity-ui/uikit';
import type {AppTheme} from '../theme';

export function AppProviders({
  theme,
  children,
}: {
  theme: AppTheme;
  children: React.ReactNode;
}) {
  return (
    <Provider theme={theme} lang="ru">
      {children}
    </Provider>
  );
}
```

- [ ] **Step 5: запустить тест и typecheck**

Run: `npx vitest run src/theme.test.ts && npm run typecheck`
Expected: PASS.

- [ ] **Step 6: коммит**

```bash
git add frontend/package.json frontend/package-lock.json \
  frontend/src/theme.ts frontend/src/theme.test.ts \
  frontend/src/providers/AppProviders.tsx
git commit -m "feat(ui): зависимости Gravity UI и модель тем"
```

Интеграция `AppProviders` в `App.tsx` и загрузка локали дат выполняются в
Task 2, чтобы каждый коммит оставлял зелёный набор тестов.

---

## Task 2: AsideHeader, меню и селектор темы

**Files:**
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/main.tsx`
- Modify: `frontend/src/Layout.tsx`
- Create: `frontend/src/components/ThemeSelect.tsx`
- Create: `frontend/src/components/ThemeDialog.tsx`
- Modify: `frontend/src/styles.css`
- Test: `frontend/src/Layout.test.tsx`

**Interfaces:**
- Consumes: `AppTheme`, `THEME_OPTIONS`.
- Produces:
  - `Layout({me, onLogout, theme, onThemeChange, children})`
  - `ThemeSelect({theme, onThemeChange, label?})`
  - `ThemeDialog({open, theme, onThemeChange, onClose})`

- [ ] **Step 1: обновить тест Layout (падающий)**

```tsx
it('показывает меню, тему, аккаунт, выход и футер', async () => {
  const onLogout = vi.fn();
  const onThemeChange = vi.fn();
  renderUI(
    <Layout me={me} onLogout={onLogout} theme="system" onThemeChange={onThemeChange}>
      <div>содержимое</div>
    </Layout>,
  );
  expect(screen.getByText('содержимое')).toBeInTheDocument();
  expect(screen.getByText('Пользователи UI')).toBeInTheDocument();
  expect(screen.getByText('Сервисные аккаунты')).toBeInTheDocument();
  expect(screen.getByText('Настройки')).toBeInTheDocument();
  expect(screen.getByText('Диагностика')).toBeInTheDocument();
  await userEvent.click(screen.getByRole('button', {name: 'Выйти'}));
  expect(onLogout).toHaveBeenCalled();
});

it('скрывает админские пункты у viewer', () => {
  renderUI(
    <Layout
      me={{...me, user: {...me.user, role: 'viewer'}}}
      onLogout={() => {}}
      theme="system"
      onThemeChange={() => {}}
    >
      <div>содержимое</div>
    </Layout>,
  );
  expect(screen.queryByText('Пользователи UI')).not.toBeInTheDocument();
  expect(screen.queryByText('Сервисные аккаунты')).not.toBeInTheDocument();
});
```

- [ ] **Step 2: запустить и убедиться, что падает**

Run: `npx vitest run src/Layout.test.tsx`
Expected: FAIL, старый topbar и `onToggleTheme`.

- [ ] **Step 3: реализовать Layout на AsideHeader**

Использовать:

```tsx
import {AsideHeader, FooterItem} from '@gravity-ui/navigation';
import {ArrowRightFromSquare, FileText, Gear, Key, Person, Persons,
  Server, ShieldCheck, Stethoscope} from '@gravity-ui/icons';
```

- `logo={{text: 'TeleMT Admin', icon: Server, onClick: () => navigate('/')}}`;
- `menuItems` с `current` по `useLocation().pathname`;
- `onItemClick: (item) => navigate(MENU_PATHS[item.id] ?? '/')`;
  `MENU_PATHS` — константа `Record<string, string>`, потому что
  `AsideHeaderItem` не обязан хранить произвольное поле `path`;
- админские пункты фильтровать по `me.user.role`;
- `compact` из `localStorage` (`tgadmin-sidebar-compact`), при отсутствии —
  `window.matchMedia('(max-width: 960px)').matches`;
- `onChangeCompact` сохраняет значение;
- `menuDensity="compact"`, `menuOverflow="scroll"`;
- `renderContent={() => <main className="page">{children}<Footer /></main>}`;
- `renderFooter`:
  - `FooterItem id="account" title={me.user.name} icon={Person} onClick={() => navigate('/account')}`;
  - `FooterItem id="logout" title="Выйти" icon={ArrowRightFromSquare} onClick={onLogout}`;
  - при `compact` — `FooterItem id="theme" title="Тема" icon={Sun} compact={compact} onClick={() => setThemeDialogOpen(true)}`;
- `aboveMenuContent` при `!compact` — `<ThemeSelect .../>`.

`ThemeSelect`:

```tsx
<Select
  aria-label="Тема"
  value={[theme]}
  options={THEME_OPTIONS.map((item) => ({value: item.value, content: item.label}))}
  onUpdate={([value]) => onThemeChange(normalizeTheme(value ?? null))}
  width="max"
/>
```

`ThemeDialog` — `Dialog` с `ThemeSelect` и кнопкой «Закрыть».

- [ ] **Step 4: удалить старый topbar CSS**

Удалить `.topbar*`, `.menu-toggle`, `.topbar-panel`, `.topbar-user` и
мобильные правила topbar. Оставить `.page`, `.app-shell` при необходимости,
`.row`, `.stack` и остальные страницы.

- [ ] **Step 5: обновить App, main и тесты**

`App.tsx`:

```ts
const [theme, setTheme] = useState<AppTheme>(() =>
  normalizeTheme(localStorage.getItem('tgadmin-theme')),
);
useEffect(() => {
  localStorage.setItem('tgadmin-theme', theme);
}, [theme]);
```

Три `<ThemeProvider>` заменить на `<AppProviders theme={theme}>`, убрать
`document.documentElement.dataset.theme`. `Layout` получает `theme` и
`onThemeChange`; убрать `onToggleTheme`. В `main.tsx` добавить:

```ts
import {settings} from '@gravity-ui/date-utils';
void settings.loadLocale('ru');
```

В `App.test.tsx` заменить клик «Переключить тему» на выбор темы через
`ThemeSelect`/`ThemeDialog` и проверять `localStorage` + `body` class
`g-root_theme_light` / `g-root_theme_dark-hc`.

- [ ] **Step 6: запустить тесты**

Run: `npx vitest run src/Layout.test.tsx src/App.test.tsx && npm run typecheck`
Expected: PASS.

- [ ] **Step 7: коммит**

```bash
git add frontend/src/Layout.tsx frontend/src/components/ThemeSelect.tsx \
  frontend/src/components/ThemeDialog.tsx frontend/src/styles.css \
  frontend/src/Layout.test.tsx frontend/src/App.tsx frontend/src/App.test.tsx \
  frontend/src/main.tsx
git commit -m "feat(ui): боковая навигация AsideHeader и выбор темы"
```

---

## Task 3: общие обёртки форм и дат

**Files:**
- Create: `frontend/src/components/FormField.tsx`
- Create: `frontend/src/components/FormSelect.tsx`
- Create: `frontend/src/components/DateFormField.tsx`
- Create: `frontend/src/components/forms.test.tsx`

**Interfaces:**
- Produces:
  - `FormField({label, hint, error, htmlFor, children})`
  - `FormSelect({label, value, options, onUpdate, disabled?, placeholder?, qa?})`
  - `DateFormField({label, value, onUpdate, withTime?, disabled?, placeholder?})`

- [ ] **Step 1: написать падающие тесты**

```tsx
it('FormSelect отдаёт строку из массива Select', async () => {
  const onUpdate = vi.fn();
  render(
    <FormSelect
      label="Период"
      value="monthly"
      options={[
        {value: 'monthly', content: 'Месяц'},
        {value: 'weekly', content: 'Неделя'},
      ]}
      onUpdate={onUpdate}
    />,
  );
  await userEvent.click(screen.getByRole('combobox', {name: 'Период'}));
  await userEvent.click(screen.getByRole('option', {name: 'Неделя'}));
  expect(onUpdate).toHaveBeenCalledWith('weekly');
});

it('DateFormField переводит DateTime в локальную строку', async () => {
  const onUpdate = vi.fn();
  render(
    <DateFormField
      label="Действует до"
      value="2026-12-31T23:59"
      withTime
      onUpdate={onUpdate}
    />,
  );
  expect(screen.getByRole('textbox', {name: 'Действует до'})).toHaveValue(
    '2026-12-31 23:59',
  );
});
```

- [ ] **Step 2: запустить и убедиться, что падает**

Run: `npx vitest run src/components/forms.test.tsx`
Expected: FAIL, модули не найдены.

- [ ] **Step 3: реализовать обёртки**

`FormField` — `<label className="field">` с `span` и контентом; если
передан `htmlFor`, использовать `<div className="field">` + `<label htmlFor>`.

`FormSelect` — `Select` с `value={value ? [value] : []}`, `onUpdate`
берёт первый элемент, `width="max"`, `aria-label={label}`.

`DateFormField`:

```tsx
import {DateField} from '@gravity-ui/date-components';
import {dateTimeParse} from '@gravity-ui/date-utils';

const format = withTime ? 'YYYY-MM-DD HH:mm' : 'YYYY-MM-DD';
const valueDate = value ? dateTimeParse(value, {format}) ?? null : null;

<DateField
  aria-label={label}
  value={valueDate}
  format={format}
  hasClear
  disabled={disabled}
  placeholder={placeholder}
  onUpdate={(next) => onUpdate(next ? next.format(format) : '')}
/>
```

Если `dateTimeParse` не принимает формат вторым аргументом, использовать
`dateTime({input: value, format})` и проверять `isValid()`. Это
единственное место преобразования дат.

- [ ] **Step 4: запустить тесты**

Run: `npx vitest run src/components/forms.test.tsx && npm run typecheck`
Expected: PASS.

- [ ] **Step 5: коммит**

```bash
git add frontend/src/components/FormField.tsx \
  frontend/src/components/FormSelect.tsx \
  frontend/src/components/DateFormField.tsx frontend/src/components/forms.test.tsx
git commit -m "feat(ui): обёртки форм и дат Gravity UI"
```

---

## Task 4: UsersPage и NewUserPage

**Files:**
- Modify: `frontend/src/pages/UsersPage.tsx`
- Modify: `frontend/src/pages/NewUserPage.tsx`
- Test: `frontend/src/pages/UsersPage.test.tsx`
- Test: `frontend/src/pages/UsersPageExtended.test.tsx`
- Test: `frontend/src/pages/NewUserPage.test.tsx`

**Interfaces:**
- Consumes: `FormSelect`, `FormField`, `DateFormField`, `Checkbox`, `TextInput`.

- [ ] **Step 1: добавить helper выбора Gravity Select**

Создать в `frontend/src/test/select.ts`:

```ts
import {screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';

export async function chooseSelect(label: string, option: string) {
  await userEvent.click(screen.getByRole('combobox', {name: label}));
  await userEvent.click(screen.getByRole('option', {name: option}));
}
```

- [ ] **Step 2: обновить тесты под Gravity Select/Checkbox**

Заменить `selectOptions(screen.getByLabelText('...'), '...')` на
`chooseSelect('...', '...')`; клики по чекбоксам — по `getByRole('checkbox', {name})`.
Ожидать те же API-тела.

- [ ] **Step 3: запустить тесты и увидеть падение**

Run: `npx vitest run src/pages/UsersPage.test.tsx src/pages/UsersPageExtended.test.tsx src/pages/NewUserPage.test.tsx`
Expected: FAIL, нативные контролы и старые селекторы.

- [ ] **Step 4: мигрировать UsersPage**

- чекбоксы выбора и фильтров — `Checkbox` с `aria-label`/`content`;
- мобильная сортировка — `Select` через `FormSelect`;
- мобильные карточки — `Checkbox`;
- остальные поля уже `TextInput`.

- [ ] **Step 5: мигрировать NewUserPage**

- период квоты — `FormSelect`;
- дата — `DateFormField withTime`;
- `sourceDeny` — `TextArea`;
- checkbox риска — `Checkbox`.

- [ ] **Step 6: запустить тесты**

Run: `npx vitest run src/pages/UsersPage.test.tsx src/pages/UsersPageExtended.test.tsx src/pages/NewUserPage.test.tsx && npm run typecheck`
Expected: PASS.

- [ ] **Step 7: коммит**

```bash
git add frontend/src/test/select.ts frontend/src/pages/UsersPage.tsx \
  frontend/src/pages/NewUserPage.tsx frontend/src/pages/UsersPage.test.tsx \
  frontend/src/pages/UsersPageExtended.test.tsx frontend/src/pages/NewUserPage.test.tsx
git commit -m "feat(ui): формы списка клиентов на Gravity UI"
```

---

## Task 5: UserPage

**Files:**
- Modify: `frontend/src/pages/UserPage.tsx`
- Test: `frontend/src/pages/UserPage.test.tsx`
- Test: `frontend/src/pages/UserPageExtended.test.tsx`

**Interfaces:**
- Consumes: `FormSelect`, `FormField`, `DateFormField`, `Checkbox`, `TextArea`.

- [ ] **Step 1: обновить тесты**

- период риска, период квоты, режим автосброса — `chooseSelect`;
- `clear*`, `riskNewCountries` — `getByRole('checkbox', {name})`;
- дата — `getByRole('textbox', {name: 'Действует до'})`;
- `sourceDeny` — `getByRole('textbox', {name: /Запрещённые сети/})`;
- день риска — кнопка с прежним текстом.

- [ ] **Step 2: запустить и увидеть падение**

Run: `npx vitest run src/pages/UserPage.test.tsx src/pages/UserPageExtended.test.tsx`
Expected: FAIL.

- [ ] **Step 3: мигрировать**

- 4 `select` → `FormSelect`;
- 6 чекбоксов → `Checkbox`;
- `datetime-local` «Действует до» → `DateFormField withTime`;
- `textarea` → `TextArea`;
- нативная кнопка дня риска → `Button view="flat" className="risk-day"`.

- [ ] **Step 4: запустить тесты**

Run: `npx vitest run src/pages/UserPage.test.tsx src/pages/UserPageExtended.test.tsx && npm run typecheck`
Expected: PASS.

- [ ] **Step 5: коммит**

```bash
git add frontend/src/pages/UserPage.tsx frontend/src/pages/UserPage.test.tsx \
  frontend/src/pages/UserPageExtended.test.tsx
git commit -m "feat(ui): формы карточки клиента на Gravity UI"
```

---

## Task 6: SettingsPage и SettingsExtras

**Files:**
- Modify: `frontend/src/pages/SettingsPage.tsx`
- Modify: `frontend/src/pages/SettingsExtras.tsx`
- Test: `frontend/src/pages/SettingsPage.test.tsx`
- Test: `frontend/src/pages/SettingsExtras.test.tsx`
- Test: `frontend/src/pages/SettingsExtrasExtended.test.tsx`

**Interfaces:**
- Consumes: `FormField`, `FormSelect`, `Checkbox`, `TextArea`.

- [ ] **Step 1: обновить тесты** под `combobox`, `checkbox`, `textbox`.
- [ ] **Step 2: запустить и увидеть падение**

Run: `npx vitest run src/pages/SettingsPage.test.tsx src/pages/SettingsExtras.test.tsx src/pages/SettingsExtrasExtended.test.tsx`
Expected: FAIL.

- [ ] **Step 3: мигрировать**

- bool-настройки → `Checkbox`;
- enum-настройки → `FormSelect`;
- `textarea` в шаблонах/webhook → `TextArea`;
- остальные поля уже `TextInput`.

- [ ] **Step 4: запустить тесты**

Run: `npx vitest run src/pages/SettingsPage.test.tsx src/pages/SettingsExtras.test.tsx src/pages/SettingsExtrasExtended.test.tsx && npm run typecheck`
Expected: PASS.

- [ ] **Step 5: коммит**

```bash
git add frontend/src/pages/SettingsPage.tsx frontend/src/pages/SettingsExtras.tsx \
  frontend/src/pages/SettingsPage.test.tsx frontend/src/pages/SettingsExtras.test.tsx \
  frontend/src/pages/SettingsExtrasExtended.test.tsx
git commit -m "feat(ui): настройки на компонентах Gravity UI"
```

---

## Task 7: AuditPage, AdminsPage, ServicesPage, ImportDialog

**Files:**
- Modify: `frontend/src/pages/AuditPage.tsx`
- Modify: `frontend/src/pages/AdminsPage.tsx`
- Modify: `frontend/src/pages/ServicesPage.tsx`
- Modify: `frontend/src/pages/ImportDialog.tsx`
- Test: page tests и `frontend/src/pages/ServiceTokensDialog.test.tsx`

**Interfaces:**
- Consumes: `FormSelect`, `DateFormField`, `Checkbox`, `TextInput`,
  `useFileInput`, `Button`.

- [ ] **Step 1: обновить тесты**

- аудит: select и date-поля через `chooseSelect` и `textbox`;
  в `AuditPageExtended.test.tsx` заменить `querySelectorAll('input[type="date"]')`
  на `getAllByRole('textbox')` внутри фильтра;
- админы: роль через `chooseSelect`;
- сервисы: права и чекбокс «Отключён»;
- импорт: `useFileInput` + кнопка «Выбрать файл», скрытый input с
  `aria-label="CSV-файл"` остаётся доступен.

- [ ] **Step 2: запустить и увидеть падение**

Run: `npx vitest run src/pages/AuditPage.test.tsx src/pages/AuditPageExtended.test.tsx src/pages/AdminsPage.test.tsx src/pages/ServicesPage.test.tsx src/pages/UsersPageExtended.test.tsx`
Expected: FAIL.

- [ ] **Step 3: мигрировать**

- AuditPage: `FormSelect`, два `DateFormField` с `withTime={false}`;
- AdminsPage: роль — `FormSelect`;
- ServicesPage: права — `FormSelect`, «Отключён» — `Checkbox`;
- ImportDialog:

```tsx
const {controlProps, triggerProps} = useFileInput({
  onUpdate: (files) => void chooseFile(files[0]),
});
return (
  <>
    <input {...controlProps} aria-label="CSV-файл" />
    <Button {...triggerProps}>Выбрать файл</Button>
  </>
);
```

- [ ] **Step 4: запустить тесты**

Run: `npx vitest run src/pages/AuditPage.test.tsx src/pages/AuditPageExtended.test.tsx src/pages/AdminsPage.test.tsx src/pages/ServicesPage.test.tsx src/pages/UsersPageExtended.test.tsx && npm run typecheck`
Expected: PASS.

- [ ] **Step 5: коммит**

```bash
git add frontend/src/pages/AuditPage.tsx frontend/src/pages/AdminsPage.tsx \
  frontend/src/pages/ServicesPage.tsx frontend/src/pages/ImportDialog.tsx \
  frontend/src/pages/AuditPage.test.tsx frontend/src/pages/AuditPageExtended.test.tsx \
  frontend/src/pages/AdminsPage.test.tsx frontend/src/pages/ServicesPage.test.tsx \
  frontend/src/pages/UsersPageExtended.test.tsx
git commit -m "feat(ui): аудит, админы, сервисы и импорт на Gravity UI"
```

---

## Task 8: preview-таблицы и оставшиеся нативные кнопки

**Files:**
- Modify: `frontend/src/pages/ImportDialog.tsx`
- Modify: `frontend/src/pages/ServiceTokensDialog.tsx`
- Modify: `frontend/src/pages/DiagnosticsPage.tsx`
- Modify: `frontend/src/tableSort.tsx`
- Test: соответствующие тесты

**Interfaces:**
- Consumes: `@gravity-ui/table` (`Table`, `useTable`, `ColumnDef`), `Button`.

- [ ] **Step 1: обновить тесты**

- таблицы импорта/токенов/диагностики по-прежнему показывают строки;
- сортировка в таблицах работает;
- день риска выбирается кнопкой.

- [ ] **Step 2: запустить и увидеть падение**

Run: `npx vitest run src/pages/UsersPageExtended.test.tsx src/pages/ServiceTokensDialog.test.tsx src/pages/DiagnosticsPage.test.tsx`
Expected: FAIL, raw `<table>`.

- [ ] **Step 3: мигрировать таблицы**

Для каждой таблицы определить `ColumnDef<T>[]` и `useTable`; обернуть в
`.table-scroll`. Для preview-таблиц использовать простые `cell`-рендеры.

- [ ] **Step 4: заменить нативные кнопки**

- `tableSort` — `Button view="flat" size="s"` с `className="th-sort"`;
- день риска — `Button view="flat" className="risk-day"` с сохранением
  `aria-pressed`.

- [ ] **Step 5: запустить тесты**

Run: `npx vitest run src/pages/UsersPageExtended.test.tsx src/pages/ServiceTokensDialog.test.tsx src/pages/DiagnosticsPage.test.tsx src/pages/UserPage.test.tsx && npm run typecheck`
Expected: PASS.

- [ ] **Step 6: коммит**

```bash
git add frontend/src/pages/ImportDialog.tsx frontend/src/pages/ServiceTokensDialog.tsx \
  frontend/src/pages/DiagnosticsPage.tsx frontend/src/tableSort.tsx \
  frontend/src/pages/UsersPageExtended.test.tsx frontend/src/pages/ServiceTokensDialog.test.tsx \
  frontend/src/pages/DiagnosticsPage.test.tsx
git commit -m "feat(ui): preview-таблицы и кнопки Gravity UI"
```

---

## Task 9: CSS, токены и адаптивность

**Files:**
- Modify: `frontend/src/styles.css`
- Modify: `frontend/src/pages/*.tsx` (только оставшиеся inline-стили)
- Test: `frontend/src/pages/otherPages.test.tsx` при необходимости

- [ ] **Step 1: удалить CSS нативных контролов**

Удалить правила для `select`, `input`, `textarea`, `.check`, `.checkbox`,
`.topbar*`, `.menu-toggle`. Оставить layout-классы, таблицы, карточки,
графики, QR/TOTP.

- [ ] **Step 2: перевести отступы на токены**

Заменить явные `px` в новых/затронутых правилах на
`var(--g-spacing-1..8)`; оставить точные размеры QR/TOTP и графиков.

- [ ] **Step 3: проверить адаптивность**

- 375px: сайдбар свёрнут, `.page` не даёт горизонтальный скролл;
- 1400px: контент не растягивается сверх `max-width`;
- таблицы скроллятся внутри `.table-scroll`;
- формы складываются в одну колонку.

- [ ] **Step 4: запустить typecheck и coverage**

Run: `npm run typecheck && npm run coverage`
Expected: PASS, покрытие не ниже порогов.

- [ ] **Step 5: коммит**

```bash
git add frontend/src/styles.css frontend/src/pages
git commit -m "style(ui): очистка CSS и токены Gravity UI"
```

---

## Task 10: e2e, документация и релиз

**Files:**
- Modify: `frontend/e2e/*.spec.ts`, `frontend/e2e/mock.ts`
- Modify: `README.md`, `docs/ARCHITECTURE.md`, `CHANGELOG.md`,
  `docs/adr/README.md`
- Create: `docs/adr/0010-aside-header-and-themes.md`
- Release: `deploy/docker-compose.yml`, `deploy/demo/*`
- Private: `docs/tgproxy.md`

- [ ] **Step 1: обновить e2e**

- вход и переходы по всем пунктам сайдбара;
- сворачивание/разворачивание;
- переключение пяти тем и сохранение после reload;
- создание клиента, импорт CSV, сервисный токен, настройки;
- мобильный viewport.

Селекторы: `getByRole('link', {name})` для меню, `getByRole('combobox')`
для Select, `getByRole('checkbox')` для Checkbox.

- [ ] **Step 2: запустить e2e**

Run: `cd frontend && npm run e2e`
Expected: PASS.

- [ ] **Step 3: документация**

- README: боковая навигация, темы, скриншоты;
- ARCHITECTURE: каркас, Provider, темы;
- CHANGELOG: 1.10.0;
- ADR-0010: AsideHeader и полный набор тем;
- приватный tgproxy: обновить раздел UI, если нужно.

- [ ] **Step 4: полный прогон**

```bash
go test ./...
go vet ./...
cd frontend
npm run typecheck
npm run coverage
npm run build
npm run e2e
```

- [ ] **Step 5: коммит**

```bash
git add frontend/e2e README.md docs CHANGELOG.md
git commit -m "test(ui): e2e и документация Gravity UI"
```

- [ ] **Step 6: релиз 1.10.0**

Обновить CHANGELOG и deploy-теги, создать release PR, дождаться CI,
смержить, поставить тег `v1.10.0`, дождаться release workflow, задеплоить
на рабочий хост, обновить приватную документацию.

---

## Самопроверка плана

- Пять тем, Provider и русская локаль — Task 1.
- AsideHeader, меню, футер, тема в свёрнутом сайдбаре — Task 2.
- Общие обёртки форм и дат — Task 3.
- Все страницы с нативными контролами — Tasks 4–7.
- Preview-таблицы и кнопки — Task 8.
- CSS, токены, адаптивность — Task 9.
- E2E, документация, релиз — Task 10.
- Плейсхолдеров и выдуманных API нет; даты используют `DateField` с
  форматом, а не несуществующий time picker.
