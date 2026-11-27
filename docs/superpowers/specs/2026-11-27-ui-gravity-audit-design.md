# UI-аудит Gravity UI: сайдбар, темы и формы

- Статус: принято
- Дата: 2026-11-27
- Целевая версия: 1.10.0
- Подход: AsideHeader, полный набор тем uikit и миграция нативных форм
  на компоненты Gravity UI

## 1. Контекст

Интерфейс TeleMT Admin использует `@gravity-ui/uikit` 8.2.0 и
`@gravity-ui/table` 2.0.0, но каркас остаётся самодельным: верхний
`topbar` с мобильным бургером, ручная тема через `data-theme`, нативные
`select`, `input`, `checkbox`, `textarea` и три семантические таблицы.
Тема поддерживает только `light` и `dark`, хотя uikit предоставляет
четыре встроенные темы и режим `system`.

Gravity UI рекомендует для каркаса `@gravity-ui/navigation` с
`AsideHeader`, а для контролов — компоненты uikit. Нативные элементы
ломают единый вид, хуже работают с темами и высоким контрастом.

## 2. Цели

1. Перевести верхнее меню на боковой `AsideHeader` как в демо Gravity UI.
2. Подключить все темы uikit: `system`, `light`, `dark`, `light-hc`,
   `dark-hc`.
3. Сделать `ThemeProvider`/`Provider` единственным источником темы.
4. Заменить все нативные формы на компоненты Gravity UI.
5. Заменить оставшиеся нативные кнопки и preview-таблицы на компоненты
   Gravity UI.
6. Убрать самодельные стили topbar и нативных контролов.
7. Сохранить текущие сценарии, права, API и тексты.
8. Обновить тесты, e2e и документацию.

## 3. Не входит в задачу

- изменение информационной архитектуры страниц и бизнес-логики;
- новый визуальный бренд, логотип или маркетинговые изображения;
- замена всех layout-классов на `Flex`/`Box`;
- миграция на другой роутер или state-менеджер;
- изменение backend, API и прав;
- тёмная/светлая тема на уровне отдельной страницы;
- пользовательские настраиваемые темы и `uikit-themer`;
- i18n интерфейса, кроме русской локали компонентов uikit и дат.

## 4. Новые зависимости

- `@gravity-ui/navigation` 8.x — `AsideHeader`, `FooterItem`.
- `@bem-react/classname` — peer dependency navigation.
- `@gravity-ui/date-components` 5.x — `DateField`, `DatePicker`.
- `@gravity-ui/date-utils` 2.x — `DateTime`, локаль `ru`.

Версии ставятся через `npm install`; после установки версии фиксируются в
`package.json` и `package-lock.json`.

## 5. Каркас приложения

`frontend/src/Layout.tsx` перестаёт быть самодельным topbar и становится
обёрткой над `AsideHeader`:

- `logo`: иконка `Server`, текст `TeleMT Admin`, клик ведёт на `/`;
- `menuItems`:
  - `clients` — `Persons`, путь `/`;
  - `audit` — `FileText`, путь `/audit`;
  - `admins` — `ShieldCheck`, путь `/admins`, только admin;
  - `services` — `Key`, путь `/services`, только admin;
  - `settings` — `Gear`, путь `/settings`, только admin;
  - `diagnostics` — `Stethoscope`, путь `/diagnostics`, только admin;
- текущий пункт вычисляется по `useLocation().pathname`;
- `onItemClick` вызывает `navigate`;
- `menuDensity="compact"`, `menuOverflow="scroll"`;
- `renderContent` рендерит `<main className="page">{children}<Footer /></main>`;
- `renderFooter`:
  - `FooterItem` аккаунта с именем пользователя и иконкой `Person`;
  - `FooterItem` выхода с иконкой `ArrowRightFromSquare`;
- `compact` хранится в `localStorage` (`tgadmin-sidebar-compact`);
- на ширине до 960px сайдбар по умолчанию свёрнут;
- 2FA-предупреждение остаётся внутри контента;
- при принудительном 2FA-ограничении (`me.require2fa.enforced`) каркас
  не используется, как и сейчас.

Селектор темы:

- в развёрнутом сайдбаре — `Select` в `aboveMenuContent`;
- в свёрнутом — `FooterItem` с иконкой текущей темы, открывающий
  компактный `Popup` с теми же пятью вариантами;
- выбор сохраняется в `tgadmin-theme`;
- `ThemeProvider`/`Provider` получает выбранное значение.

Верхний `topbar`, `menu-toggle`, `topbar-panel` и связанные CSS-классы
удаляются.

## 6. Темы и провайдеры

В `App.tsx` вводится тип:

```ts
type AppTheme = 'system' | 'light' | 'dark' | 'light-hc' | 'dark-hc';
```

Правила:

- сохранённое значение валидируется по списку; неизвестное → `system`;
- по умолчанию `system`;
- `Provider` из uikit получает `theme` и `lang="ru"`;
- для дат один раз вызывается `settings.loadLocale('ru')`;
- `document.documentElement.dataset.theme` больше не используется;
- тема применяется на экранах загрузки, входа, 2FA-ограничения и в
  основном приложении через один общий провайдер, чтобы не дублировать
  `ThemeProvider` в трёх местах.

Выбор темы:

- `system` — «Как в системе»;
- `light` — «Светлая»;
- `dark` — «Тёмная»;
- `light-hc` — «Светлая контрастная»;
- `dark-hc` — «Тёмная контрастная».

## 7. Миграция форм

Все нативные контролы заменяются на компоненты uikit:

| Сейчас | Где | Станет |
|---|---|---|
| `<select>` | UsersPage, UserPage (4), SettingsPage, SettingsExtras, NewUserPage (2), ServicesPage, AuditPage, AdminsPage | `Select` |
| `checkbox` | UsersPage (6), UserPage (6), SettingsPage (1), SettingsExtras (1), ServicesPage (1), NewUserPage (1) | `Checkbox` |
| `textarea` | UserPage, SettingsExtras, NewUserPage | `TextArea` |
| текстовые/числовые `input` | SettingsPage, UsersPage (сортировка), ImportDialog | `TextInput` |
| `input[type=file]` | ImportDialog | `useFileInput` + `Button` |
| `input[type=date]` | AuditPage (2) | `DateField` с `format="YYYY-MM-DD"` |
| `input[type=datetime-local]` | UserPage, NewUserPage | `DateField` с `format="YYYY-MM-DD HH:mm"` |
| нативная `<button>` | tableSort, UserPage (день риска) | `Button` |

Для повторяющихся полей создаются обёртки:

- `FormField` — подпись, контрол, подсказка/ошибка;
- `FormSelect` — `Select` с `value: string[]` и преобразованием в строку;
- `ThemeSelect` — пять тем и подпись;
- `DateFormField` — `DateField` и преобразование `DateTime ↔ локальная
  строка`; для выбора даты из календаря используется `DatePicker`.

Правила преобразования дат:

- дата без времени хранится как `YYYY-MM-DD`;
- дата со временем — как локальная строка `YYYY-MM-DDTHH:mm`, которую
  сейчас отправляет backend;
- пустое значение означает «не задано»;
- невалидный ввод не отправляется, поле показывает ошибку.

## 8. Таблицы, кнопки и прочее

- Три preview-таблицы (`ImportDialog`, `ServiceTokensDialog`,
  `DiagnosticsPage`) переводятся на `@gravity-ui/table` с `useTable` и
  типизированными колонками.
- Таблицы данных (`UsersPage`, `AdminsPage`, `ServicesPage`) уже на
  `@gravity-ui/table`, их стиль не меняется.
- `Button` используется для сортировки и выбора дня риска; визуальные
  классы сохраняются только для размеров/состояний.
- QR и TOTP-изображения остаются на белом фоне: это требование
  сканируемости, а не темы. Исключение документируется комментарием.
- Нативные `table`, `button`, `select`, `input`, `textarea` в `src`
  после миграции не остаются, кроме скрытого input внутри официальных
  хуков uikit (`useFileInput`) и случаев, где компонент uikit сам
  рендерит нативный элемент.

## 9. CSS и токены

- Удаляются `.topbar*`, `.menu-toggle`, стили нативных `select`,
  `input`, `textarea`, `.check`, `.checkbox`.
- Остаются layout-классы страниц, таблиц, карточек, графиков и
  специальные размеры (QR, TOTP).
- Отступы и радиусы переводятся на `--g-spacing-*` и семантические
  токены там, где это не меняет сетку.
- Цвета — только `--g-*`; жёстко заданный `#fff` остаётся только под
  QR/TOTP с комментарием.
- Инлайновые `style={{...}}` сокращаются до случаев динамической ширины
  и специальных размеров.

## 10. Доступность

- Все поля имеют доступные подписи: `label` у `TextInput`/`Select` или
  `FormField` с `htmlFor`.
- `Checkbox` получает текст через `content` или children.
- `AsideHeader` управляется `compact`/`onChangeCompact`.
- Иконки в меню передаются компонентом, а не строкой.
- Проверяются клавиатурная навигация, фокус в сайдбаре и контраст в
  `light-hc`/`dark-hc`.
- Узкий viewport 375px и широкий 1400px не дают горизонтального
  переполнения страницы (таблицы скроллятся внутри `.table-scroll`).

## 11. Тесты

### Unit/UI

- `Layout` показывает пункты меню по роли, текущий пункт, аккаунт и
  выход; сворачивание сохраняется.
- `App` применяет все пять тем, валидирует сохранённое значение,
  переключает и сохраняет.
- `FormSelect`/`FormField`/`DateFormField` преобразуют значения.
- `ImportDialog` и `ServiceTokensDialog` рендерят таблицы и действия.
- Обновляются все тесты, где использовался `selectOptions`: вместо
  нативного select выбирается опция Gravity `Select`.
- Проверяется отсутствие нативных контролов в отрендеренном DOM там, где
  это возможно.

### E2E

- Вход и переходы по всем пунктам сайдбара.
- Сворачивание/разворачивание и сохранение состояния.
- Переключение всех пяти тем и сохранение после перезагрузки.
- Создание клиента, импорт CSV, сервисный токен, настройки — с новыми
  контролами.
- Мобильный viewport: сайдбар свёрнут, контент доступен.

### Проверки

- `npm run typecheck`;
- `npm run coverage` — пороги 85/85/85/70;
- `npm run build`;
- `npm run e2e`;
- `go test ./...` и `go vet ./...` без изменений backend.

## 12. Документация и поставка

- README: раздел о темах и боковой навигации, скриншоты при необходимости.
- `docs/ARCHITECTURE.md`: UI-каркас и темизация.
- `docs/CHANGELOG.md`: раздел 1.10.0.
- ADR-0010: переход на AsideHeader и полный набор тем.
- Релиз 1.10.0, деплой на рабочий хост, обновление приватного
  `docs/tgproxy.md`.

## 13. Риски и порядок

Основной риск — широкая миграция форм ломает существующие UI-тесты и
e2e-селекторы. Поэтому порядок:

1. зависимости и провайдеры/темы;
2. AsideHeader и CSS каркаса;
3. общие обёртки форм;
4. миграция страниц по одной;
5. preview-таблицы и кнопки;
6. тесты, e2e, документация, релиз.

После каждого этапа — typecheck и точечные тесты; полный e2e перед PR.
