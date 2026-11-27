#!/usr/bin/env python3
"""Генерирует русские описания параметров прокси для фронтенда.

Источники:
  1. Официальный справочник telemt CONFIG_PARAMS.ru.md — описания,
     которые уже переведены командой telemt.
  2. hack/proxy-descriptions.ru.extra.json — ручные описания для полей,
     которых нет в справочнике (в основном WEB и динамические пути).

Запуск:
  python3 hack/gen-proxy-descriptions.py \
    --docs /path/to/CONFIG_PARAMS.ru.md \
    --schema /path/to/config-schema.json \
    --extra hack/proxy-descriptions.ru.extra.json \
    --out frontend/src/i18n/proxy.ru.ts
"""

import argparse
import json
import re
import sys


def parse_docs(path):
    lines = open(path, encoding="utf-8").read().splitlines()
    section = ""
    out = {}
    for i, raw in enumerate(lines):
        line = raw.rstrip()
        m = re.match(r"^# \[([^\]]+)\]", line)
        if m:
            section = m.group(1).strip().strip("[]")
            continue
        m = re.match(r"^## (.+)$", line)
        if not m or "**" in line:
            continue
        name = m.group(1).strip().strip("[]").strip()
        full = name if "." in name else (f"{section}.{name}" if section else name)
        for j in range(i + 1, min(i + 20, len(lines))):
            nxt = lines[j].strip()
            if nxt.startswith("## ") or nxt.startswith("# ["):
                break
            md = re.match(r"^- \*\*Описание\*\*:\s*(.+)$", nxt)
            if md:
                out[full] = md.group(1).strip()
                break
    return out


def is_container(field):
    return field.get("value", {}).get("type") in ("array", "object")


def has_leaf(field, fields):
    prefix = field["path"] + "."
    return any(
        other["path"].startswith(prefix)
        and "[]" not in other["path"]
        and "{" not in other["path"]
        and not is_container(other)
        for other in fields
    )


def ui_rows(fields):
    writable = [
        f
        for f in fields
        if f.get("api", {}).get("config_write")
        and "[]" not in f["path"]
        and "{" not in f["path"]
    ]
    readonly = [
        f
        for f in fields
        if not f.get("api", {}).get("config_write")
        and "[]" not in f["path"]
        and "{" not in f["path"]
    ]
    rows = [
        f for f in writable if not is_container(f) or not has_leaf(f, writable)
    ]
    rows += [f for f in readonly if not is_container(f) or not has_leaf(f, readonly)]
    return rows


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--docs", required=True)
    parser.add_argument("--schema", required=True)
    parser.add_argument("--extra", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    docs = parse_docs(args.docs)
    extra = json.load(open(args.extra, encoding="utf-8"))
    schema = json.load(open(args.schema, encoding="utf-8"))["data"]["fields"]

    merged = dict(docs)
    merged.update(extra)
    schema_paths = {f["path"] for f in schema if f.get("description")}
    merged = {path: text for path, text in merged.items() if path in schema_paths}

    missing = [f["path"] for f in ui_rows(schema) if not merged.get(f["path"])]
    if missing:
        print("Нет русского описания для полей:", file=sys.stderr)
        for path in missing:
            print("  " + path, file=sys.stderr)
        return 1

    lines = [
        "// Сгенерировано hack/gen-proxy-descriptions.py.",
        "// Источник: telemt CONFIG_PARAMS.ru.md и hack/proxy-descriptions.ru.extra.json.",
        "// Не редактировать вручную.",
        "",
        "export const proxyDescriptionsRu: Record<string, string> = {",
    ]
    for path in sorted(merged):
        lines.append(f"  {json.dumps(path, ensure_ascii=False)}: {json.dumps(merged[path], ensure_ascii=False)},")
    lines.append("};")
    lines.append("")

    open(args.out, "w", encoding="utf-8").write("\n".join(lines))
    print(f"записано {len(merged)} описаний в {args.out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
