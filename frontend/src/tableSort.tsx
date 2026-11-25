import { useState } from 'react';

export type SortRule = { id: string; desc: boolean };

/** Общее поведение сортировки: возрастание, убывание, выключить. */
export function useTableSort(initial: SortRule[] = []) {
  const [sorting, setSorting] = useState<SortRule[]>(initial);

  const toggleSort = (id: string) => {
    setSorting((prev) => {
      const index = prev.findIndex((item) => item.id === id);
      if (index === -1) return [...prev, { id, desc: false }];
      if (!prev[index].desc) {
        return prev.map((item, i) => (i === index ? { ...item, desc: true } : item));
      }
      return prev.filter((_, i) => i !== index);
    });
  };

  const sortHeader = (id: string, label: string) => {
    const index = sorting.findIndex((item) => item.id === id);
    const active = index !== -1;
    return (
      <button
        type="button"
        className={`th-sort${active ? ' th-sort-active' : ''}`}
        onClick={() => toggleSort(id)}
        title="Сортировать: по возрастанию, по убыванию, выключить"
      >
        <span>{label}</span>
        {active && (
          <span className="th-sort-mark">
            {sorting[index].desc ? '↓' : '↑'}
            {sorting.length > 1 ? index + 1 : ''}
          </span>
        )}
      </button>
    );
  };

  const sortRows = <T,>(rows: T[], compare: (a: T, b: T, id: string) => number): T[] => {
    if (sorting.length === 0) return rows;
    return [...rows].sort((a, b) => {
      for (const rule of sorting) {
        const result = compare(a, b, rule.id);
        if (result !== 0) return rule.desc ? -result : result;
      }
      return 0;
    });
  };

  return { sorting, setSorting, toggleSort, sortHeader, sortRows };
}
