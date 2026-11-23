#!/usr/bin/env sh
# Проверяет, что в публичном репозитории нет данных нашей инфраструктуры.
set -eu
cd "$(dirname "$0")/.."
if git grep -n -i -E "q1w2e3|pantheon|mcconfig|10\.0\.0\.9|37\.192\.|/home/mcconfig" -- . ':!go.sum' ':!hack/check-anonymized.sh'; then
  echo "найдены данные инфраструктуры" >&2
  exit 1
fi
echo "обезличивание: чисто"
