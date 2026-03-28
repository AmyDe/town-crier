import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { createRequiredContext } from '../../utils/createRequiredContext';

describe('createRequiredContext', () => {
  it('throws when hook is called outside provider', () => {
    const [, useValue] = createRequiredContext<string>('TestContext');

    expect(() => {
      renderHook(() => useValue());
    }).toThrow('useTestContext must be used within a TestContext.Provider');
  });
});
