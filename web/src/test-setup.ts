import '@testing-library/jest-dom/vitest';

// Node 22+ ships a built-in localStorage that shadows jsdom's Storage.
// Polyfill a spec-compliant in-memory Storage on `window` so tests behave
// the same way as a real browser.
function createMemoryStorage(): Storage {
  let store: Record<string, string> = {};
  return {
    get length() {
      return Object.keys(store).length;
    },
    clear() {
      store = {};
    },
    getItem(key: string) {
      return Object.prototype.hasOwnProperty.call(store, key) ? store[key]! : null;
    },
    key(index: number) {
      return Object.keys(store)[index] ?? null;
    },
    removeItem(key: string) {
      delete store[key];
    },
    setItem(key: string, value: string) {
      store[key] = String(value);
    },
  };
}

Object.defineProperty(window, 'localStorage', {
  value: createMemoryStorage(),
  writable: true,
  configurable: true,
});
