import { renderHook } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { useMedia } from './useMedia';

describe('useMedia', () => {
  it('возвращает matches из matchMedia', () => {
    vi.stubGlobal('matchMedia', (query: string) => ({
      matches: true,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    }));
    const { result } = renderHook(() => useMedia('(max-width: 760px)'));
    expect(result.current).toBe(true);
  });
});
