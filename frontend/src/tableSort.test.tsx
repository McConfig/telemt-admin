import { renderHook, act } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { useTableSort } from './tableSort';

type Row = { name: string; size: number };
const compare = (a: Row, b: Row, id: string) =>
  id === 'size' ? a.size - b.size : a.name.localeCompare(b.name);

describe('useTableSort', () => {
  it('циклически переключает возрастание, убывание и сброс', () => {
    const { result } = renderHook(() => useTableSort());
    expect(result.current.sorting).toEqual([]);

    act(() => result.current.toggleSort('size'));
    expect(result.current.sorting).toEqual([{ id: 'size', desc: false }]);

    act(() => result.current.toggleSort('size'));
    expect(result.current.sorting).toEqual([{ id: 'size', desc: true }]);

    act(() => result.current.toggleSort('size'));
    expect(result.current.sorting).toEqual([]);
  });

  it('поддерживает несколько правил и сортирует по ним', () => {
    const { result } = renderHook(() => useTableSort());
    act(() => result.current.toggleSort('size'));
    act(() => result.current.toggleSort('name'));
    expect(result.current.sorting).toEqual([
      { id: 'size', desc: false },
      { id: 'name', desc: false },
    ]);
    const rows: Row[] = [
      { name: 'b', size: 2 },
      { name: 'a', size: 1 },
      { name: 'c', size: 1 },
    ];
    expect(result.current.sortRows(rows, compare).map((r) => r.name)).toEqual(['a', 'c', 'b']);
    act(() => result.current.toggleSort('size'));
    expect(result.current.sortRows(rows, compare).map((r) => r.name)).toEqual(['b', 'a', 'c']);
  });
});
