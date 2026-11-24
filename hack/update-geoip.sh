#!/usr/bin/env bash
# Скачивает бесплатные базы DB-IP Lite (город и ASN) и раскладывает их в
# каталог, который монтируется в контейнер tgadmin.
#
# Использование:
#   bash hack/update-geoip.sh [каталог]
#
# По умолчанию каталог ./geoip. Файлы называются так, как их ждут настройки
# TGADMIN_GEOIP_CITY и TGADMIN_GEOIP_ASN:
#   geoip/GeoLite2-City.mmdb
#   geoip/GeoLite2-ASN.mmdb
#
# Базы DB-IP Lite распространяются по лицензии CC BY 4.0, регистрация и
# лицензионный ключ не нужны. Обновляются раз в месяц; достаточно запускать
# скрипт раз в месяц по cron.
set -euo pipefail

dir="${1:-geoip}"
mkdir -p "$dir"
month="$(date -u +%Y-%m)"
base="https://download.db-ip.com/free"

fetch() {
  local name="$1" target="$2"
  local url="$base/$name-$month.mmdb.gz"
  local tmp
  tmp="$(mktemp "${dir}/.${target}.XXXXXX")"
  echo "скачиваю $url"
  if ! curl -fsSL --retry 3 --retry-delay 5 -o "$tmp" "$url"; then
    rm -f "$tmp"
    return 1
  fi
  echo "распаковываю в ${dir}/${target}"
  if ! gzip -dc "$tmp" > "${dir}/${target}.new"; then
    rm -f "$tmp" "${dir}/${target}.new"
    return 1
  fi
  rm -f "$tmp"
  mv "${dir}/${target}.new" "${dir}/${target}"
}

# Если базы за текущий месяц ещё не опубликованы, пробуем предыдущий.
if ! fetch dbip-city-lite "GeoLite2-City.mmdb"; then
  echo "база за $month недоступна, пробую предыдущий месяц"
  month="$(date -u -v-1m +%Y-%m 2>/dev/null || date -u -d '1 month ago' +%Y-%m)"
  fetch dbip-city-lite "GeoLite2-City.mmdb"
fi
if ! fetch dbip-asn-lite "GeoLite2-ASN.mmdb"; then
  echo "база ASN за $month недоступна, пробую предыдущий месяц"
  month="$(date -u -v-1m +%Y-%m 2>/dev/null || date -u -d '1 month ago' +%Y-%m)"
  fetch dbip-asn-lite "GeoLite2-ASN.mmdb"
fi

ls -lh "$dir"/GeoLite2-City.mmdb "$dir"/GeoLite2-ASN.mmdb
echo "готово. Перезапустите tgadmin, чтобы он перечитал базы:"
echo "  docker compose up -d --force-recreate tgadmin"
