export function formatBytes(n: number): string {
  if (n < 1024) return `${n} Б`;
  const units = ['КиБ', 'МиБ', 'ГиБ', 'ТиБ'];
  let value = n / 1024;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${value.toFixed(1)} ${units[i]}`;
}

export function formatTime(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString('ru-RU');
}
