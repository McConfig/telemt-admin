# ADR-0011: WebAuthn/passkeys как второй фактор

- Статус: принято
- Дата: 2026-11-27
- Контекст: версия 1.11.0

## Проблема

TOTP-код удобен, но его можно перехватить фишинговой страницей, а
резервные коды легко потерять. Нужен второй фактор, устойчивый к
фишингу и привязанный к устройству или платформе.

## Решение

1. Passkey — второй фактор после пароля. Вход без пароля не вводится.
2. Использовать `go-webauthn/webauthn` и `@simplewebauthn/browser`.
3. Хранить публичные credential records в Valkey:
   `auth:webauthn:<user>`, `auth:webauthn_cred:<id>`; challenge —
   `auth:webauthn_register:<session>` и `auth:webauthn_login:<pending>`.
4. RP ID и origin брать из `TGADMIN_BASE_URL`; без HTTPS/localhost
   WebAuthn выключен.
5. Разрешить до 10 passkeys; регистрация и удаление требуют текущий
   пароль.
6. Считать passkey полноценной 2FA: проверки обязательной 2FA учитывают
   passkey наравне с TOTP.
7. TOTP и резервные коды остаются резервным путём.
8. Администраторский сброс 2FA удаляет и TOTP, и passkeys.
9. Аудит: `passkey.create`, `passkey.delete`, `login.webauthn`,
   `admin.mfa_reset`.

## Альтернативы

1. **Своя реализация WebAuthn.** Высокий риск ошибок в CBOR/COSE,
   challenge и origin.
2. **Passkey вместо пароля.** Удобнее, но сложнее и рискованнее для
   единственного администратора.
3. **Хранить credentials в hash пользователя.** Раздувает запись и
   усложняет конкурентные обновления.
4. **Оставить только TOTP.** Не решает фишинг и потерю устройства.

## Последствия

- Новые ключи `auth:webauthn*`; миграции нет.
- Откат на 1.10.x безопасен: passkeys игнорируются, TOTP работает.
- Passkey не обходит CIDR allowlist и не работает для сервисных токенов.
- Регистрация passkey требует HTTPS; localhost разрешён для разработки.
- Полный e2e с virtual authenticator проверяет регистрацию, вход и
  удаление passkey на реальном стеке (v1.11.1).
