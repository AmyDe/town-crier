import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import type { ReactNode } from 'react';
import { createRequiredContext } from '../../utils/createRequiredContext';

describe('createRequiredContext', () => {
  it('throws when hook is called outside provider', () => {
    const [, useValue] = createRequiredContext<string>('TestContext');

    expect(() => {
      renderHook(() => useValue());
    }).toThrow('useTestContext must be used within a TestContext.Provider');
  });

  it('returns value when hook is called inside provider', () => {
    const [Provider, useValue] = createRequiredContext<string>('TestContext');

    function wrapper({ children }: { children: ReactNode }) {
      return <Provider value="hello">{children}</Provider>;
    }

    const { result } = renderHook(() => useValue(), { wrapper });

    expect(result.current).toBe('hello');
  });
});
