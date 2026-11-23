#!/usr/bin/env bash
# Готовит deploy/telemt/config.toml: секрет, домен и порт.
#
# Использование:
#   DOMAIN=proxy.example.com PORT=8444 bash deploy/telemt-init.sh
#
# Повторный запуск не меняет существующий секрет, если файл deploy/telemt/secret
# уже есть. Ссылка печатается в конце.
set -euo pipefail

DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
TDIR="$DIR/telemt"
SECRET_FILE="$TDIR/secret"
CONFIG="$TDIR/config.toml"
TEMPLATE="$TDIR/config.toml.tmpl"
DOMAIN=${DOMAIN:-proxy.example.com}
PORT=${PORT:-8444}

mkdir -p "$TDIR"
if [ -s "$SECRET_FILE" ]; then
  echo "секрет уже есть: $SECRET_FILE"
else
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 16 > "$SECRET_FILE"
  else
    od -An -tx1 -N16 /dev/urandom | tr -d ' \n' > "$SECRET_FILE"
  fi
  chmod 600 "$SECRET_FILE"
  echo "секрет создан: $SECRET_FILE"
fi
SECRET=$(tr -d '[:space:]' < "$SECRET_FILE")
if ! printf '%s' "$SECRET" | grep -Eq '^[0-9a-f]{32}$'; then
  echo "ошибка: секрет должен быть 32 hex-символа" >&2
  exit 1
fi
sed -e "s|__SECRET__|$SECRET|" -e "s|__DOMAIN__|$DOMAIN|" -e "s|__PORT__|$PORT|" "$TEMPLATE" > "$CONFIG"
chmod 600 "$CONFIG"
echo "конфиг собран: $CONFIG"
DOMAIN_HEX=$(printf '%s' "$DOMAIN" | od -An -tx1 | tr -d ' \n')
echo "ссылка: tg://proxy?server=$DOMAIN&port=$PORT&secret=ee${SECRET}${DOMAIN_HEX}"
