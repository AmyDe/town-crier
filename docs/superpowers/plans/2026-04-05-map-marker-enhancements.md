# Map Marker Enhancements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Color-code map markers by saved state (amber=saved, grey=unsaved), add save/unsave from popups, and auto-fit bounds prioritising saved applications.

**Architecture:** Extend MapPort with saved-application methods backed by existing `savedApplicationsApi`. Enhance `useMapData` to fetch saved UIDs and expose optimistic save/unsave mutations. Replace default Leaflet markers with custom SVG DivIcons. Add FitBounds component following FitToCircle pattern.

**Tech Stack:** React, react-leaflet, Leaflet DivIcon (SVG), CSS Modules, Vitest

**Spec:** `docs/superpowers/specs/2026-04-05-map-marker-enhancements-design.md`

---

## File Map

| Action | File | Responsibility |
|--------|------|---------------|
| Modify | `src/domain/ports/map-port.ts` | Add saved-app methods to port interface |
| Modify | `src/features/Map/__tests__/spies/spy-map-port.ts` | Spy implementations for new methods |
| Modify | `src/features/Map/__tests__/fixtures/map.fixtures.ts` | Add `aSavedApplication` fixture |
| Modify | `src/features/Map/ApiMapAdapter.ts` | Wire new methods to `savedApplicationsApi` |
| Modify | `src/features/Map/useMapData.ts` | Fetch saved UIDs, expose save/unsave mutations |
| Modify | `src/features/Map/__tests__/useMapData.test.ts` | Tests for saved UIDs fetch + mutations |
| Create | `src/features/Map/markerIcons.ts` | SVG-based DivIcon factory for saved/unsaved |
| Create | `src/features/Map/FitBounds.tsx` | Auto-fit map bounds child component |
| Create | `src/features/Map/BookmarkButton.tsx` | Save/unsave toggle button with SVG icon |
| Create | `src/features/Map/BookmarkButton.module.css` | Bookmark button styling |
| Modify | `src/features/Map/MapPage.tsx` | Wire new components, replace default markers |
| Modify | `src/features/Map/MapPage.module.css` | Add popup header layout |
| Modify | `src/features/Map/__tests__/MapPage.test.tsx` | Tests for save/unsave, bookmark buttons |

All paths relative to `web/`.

---

### Task 1: Extend MapPort and SpyMapPort

**Files:**
- Modify: `src/domain/ports/map-port.ts`
- Modify: `src/features/Map/__tests__/spies/spy-map-port.ts`

- [ ] **Step 1: Add methods to MapPort interface**

```typescript
import type { ApplicationUid, AuthorityId, AuthorityListItem, PlanningApplication, SavedApplication } from '../types';

export interface MapPort {
  fetchMyAuthorities(): Promise<readonly AuthorityListItem[]>;
  fetchApplicationsByAuthority(authorityId: AuthorityId): Promise<readonly PlanningApplication[]>;
  fetchSavedApplications(): Promise<readonly SavedApplication[]>;
  saveApplication(uid: ApplicationUid): Promise<void>;
  unsaveApplication(uid: ApplicationUid): Promise<void>;
}
```

- [ ] **Step 2: Update SpyMapPort with spy implementations**

```typescript
import type { ApplicationUid, AuthorityId, AuthorityListItem, PlanningApplication, SavedApplication } from '../../../../domain/types';
import type { MapPort } from '../../../../domain/ports/map-port';

export class SpyMapPort implements MapPort {
  fetchMyAuthoritiesCalls = 0;
  fetchMyAuthoritiesResult: readonly AuthorityListItem[] = [];
  fetchMyAuthoritiesError: Error | null = null;

  async fetchMyAuthorities(): Promise<readonly AuthorityListItem[]> {
    this.fetchMyAuthoritiesCalls++;
    if (this.fetchMyAuthoritiesError) {
      throw this.fetchMyAuthoritiesError;
    }
    return this.fetchMyAuthoritiesResult;
  }

  fetchApplicationsByAuthorityCalls: AuthorityId[] = [];
  fetchApplicationsByAuthorityResults: Map<number, readonly PlanningApplication[]> = new Map();
  fetchApplicationsByAuthorityError: Error | null = null;

  async fetchApplicationsByAuthority(authorityId: AuthorityId): Promise<readonly PlanningApplication[]> {
    this.fetchApplicationsByAuthorityCalls.push(authorityId);
    if (this.fetchApplicationsByAuthorityError) {
      throw this.fetchApplicationsByAuthorityError;
    }
    return this.fetchApplicationsByAuthorityResults.get(authorityId as number) ?? [];
  }

  fetchSavedApplicationsCalls = 0;
  fetchSavedApplicationsResult: readonly SavedApplication[] = [];
  fetchSavedApplicationsError: Error | null = null;

  async fetchSavedApplications(): Promise<readonly SavedApplication[]> {
    this.fetchSavedApplicationsCalls++;
    if (this.fetchSavedApplicationsError) {
      throw this.fetchSavedApplicationsError;
    }
    return this.fetchSavedApplicationsResult;
  }

  saveApplicationCalls: ApplicationUid[] = [];
  saveApplicationError: Error | null = null;

  async saveApplication(uid: ApplicationUid): Promise<void> {
    this.saveApplicationCalls.push(uid);
    if (this.saveApplicationError) {
      throw this.saveApplicationError;
    }
  }

  unsaveApplicationCalls: ApplicationUid[] = [];
  unsaveApplicationError: Error | null = null;

  async unsaveApplication(uid: ApplicationUid): Promise<void> {
    this.unsaveApplicationCalls.push(uid);
    if (this.unsaveApplicationError) {
      throw this.unsaveApplicationError;
    }
  }
}
```

- [ ] **Step 3: Verify types compile**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add src/domain/ports/map-port.ts src/features/Map/__tests__/spies/spy-map-port.ts
git commit -m "feat(web): extend MapPort with saved-application methods"
```

---

### Task 2: Wire ApiMapAdapter

**Files:**
- Modify: `src/features/Map/ApiMapAdapter.ts`

- [ ] **Step 1: Import savedApplicationsApi and implement new methods**

```typescript
import type { ApiClient } from '../../api/client';
import type { ApplicationUid, AuthorityId, AuthorityListItem, PlanningApplication, SavedApplication } from '../../domain/types';
import type { MapPort } from '../../domain/ports/map-port';
import { applicationsApi } from '../../api/applications';
import { savedApplicationsApi } from '../../api/savedApplications';

export class ApiMapAdapter implements MapPort {
  private readonly apps: ReturnType<typeof applicationsApi>;
  private readonly saved: ReturnType<typeof savedApplicationsApi>;

  constructor(client: ApiClient) {
    this.apps = applicationsApi(client);
    this.saved = savedApplicationsApi(client);
  }

  async fetchMyAuthorities(): Promise<readonly AuthorityListItem[]> {
    return this.apps.getMyAuthorities();
  }

  async fetchApplicationsByAuthority(authorityId: AuthorityId): Promise<readonly PlanningApplication[]> {
    return this.apps.getByAuthority(authorityId as number);
  }

  async fetchSavedApplications(): Promise<readonly SavedApplication[]> {
    return this.saved.list();
  }

  async saveApplication(uid: ApplicationUid): Promise<void> {
    await this.saved.save(uid as string);
  }

  async unsaveApplication(uid: ApplicationUid): Promise<void> {
    await this.saved.remove(uid as string);
  }
}
```

- [ ] **Step 2: Verify types compile**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add src/features/Map/ApiMapAdapter.ts
git commit -m "feat(web): wire saved-application methods in ApiMapAdapter"
```

---

### Task 3: TDD useMapData — Fetch Saved UIDs

**Files:**
- Modify: `src/features/Map/__tests__/fixtures/map.fixtures.ts`
- Modify: `src/features/Map/__tests__/useMapData.test.ts`
- Modify: `src/features/Map/useMapData.ts`

- [ ] **Step 1: Add saved application fixture**

Append to `map.fixtures.ts`:

```typescript
import type { AuthorityListItem, PlanningApplication, SavedApplication } from '../../../../domain/types';
import { asAuthorityId, asApplicationUid } from '../../../../domain/types';

// ... existing fixtures unchanged ...

export function aSavedApplication(overrides?: Partial<SavedApplication>): SavedApplication {
  return {
    applicationUid: asApplicationUid('app-001'),
    savedAt: '2026-03-15T10:00:00Z',
    application: {
      uid: asApplicationUid('app-001'),
      name: 'Application 1',
      address: '12 Mill Road, Cambridge',
      postcode: 'CB1 2AD',
      description: 'Erection of two-storey rear extension',
      appType: 'Full Planning',
      appState: 'Undecided',
      areaName: 'Cambridge City Council',
      startDate: '2026-01-15',
      url: null,
    },
    ...overrides,
  };
}
```

- [ ] **Step 2: Write failing test — fetches saved UIDs alongside applications**

Add to `useMapData.test.ts`:

```typescript
import { aSavedApplication } from './fixtures/map.fixtures';

// ... inside describe('useMapData') ...

it('fetches saved application UIDs alongside applications', async () => {
  const spy = new SpyMapPort();
  const auth = anAuthority();
  const app = anApplication();
  spy.fetchMyAuthoritiesResult = [auth];
  spy.fetchApplicationsByAuthorityResults.set(1, [app]);
  spy.fetchSavedApplicationsResult = [aSavedApplication()];

  const { result } = renderHook(() => useMapData(spy));

  await waitFor(() => {
    expect(result.current.isLoading).toBe(false);
  });

  expect(result.current.savedUids.has(asApplicationUid('app-001'))).toBe(true);
  expect(result.current.savedUids.size).toBe(1);
  expect(spy.fetchSavedApplicationsCalls).toBe(1);
});

it('returns empty savedUids when no applications are saved', async () => {
  const spy = new SpyMapPort();
  spy.fetchMyAuthoritiesResult = [anAuthority()];
  spy.fetchApplicationsByAuthorityResults.set(1, [anApplication()]);
  spy.fetchSavedApplicationsResult = [];

  const { result } = renderHook(() => useMapData(spy));

  await waitFor(() => {
    expect(result.current.isLoading).toBe(false);
  });

  expect(result.current.savedUids.size).toBe(0);
});
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd web && npx vitest run src/features/Map/__tests__/useMapData.test.ts`
Expected: FAIL — `savedUids` not returned from `useMapData`

- [ ] **Step 4: Implement — fetch saved apps in parallel, return savedUids**

Replace `useMapData.ts` with:

```typescript
import type { ApplicationUid, PlanningApplication } from '../../domain/types';
import type { MapPort } from '../../domain/ports/map-port';
import { useFetchData } from '../../hooks/useFetchData';

interface MapData {
  readonly applications: readonly PlanningApplication[];
  readonly fetchedSavedUids: ReadonlySet<ApplicationUid>;
}

export function useMapData(port: MapPort) {
  const { data, isLoading, error, refresh } = useFetchData<MapData>(
    async () => {
      const [authorities, savedApps] = await Promise.all([
        port.fetchMyAuthorities(),
        port.fetchSavedApplications(),
      ]);

      const uniqueAuthorityIds = [...new Set(authorities.map(a => a.id))];
      const applicationArrays = await Promise.all(
        uniqueAuthorityIds.map(id => port.fetchApplicationsByAuthority(id)),
      );

      return {
        applications: applicationArrays.flat(),
        fetchedSavedUids: new Set(savedApps.map(s => s.applicationUid)),
      };
    },
    [port],
  );

  const savedUids: ReadonlySet<ApplicationUid> = data?.fetchedSavedUids ?? new Set();

  return {
    applications: data?.applications ?? [],
    savedUids,
    isLoading,
    error,
    refresh,
  };
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd web && npx vitest run src/features/Map/__tests__/useMapData.test.ts`
Expected: All tests PASS

- [ ] **Step 6: Commit**

```bash
git add src/features/Map/__tests__/fixtures/map.fixtures.ts src/features/Map/__tests__/useMapData.test.ts src/features/Map/useMapData.ts
git commit -m "feat(web): fetch saved application UIDs in useMapData"
```

---

### Task 4: TDD useMapData — Save/Unsave Mutations

**Files:**
- Modify: `src/features/Map/__tests__/useMapData.test.ts`
- Modify: `src/features/Map/useMapData.ts`

- [ ] **Step 1: Write failing tests for save/unsave**

Add to `useMapData.test.ts`:

```typescript
import { renderHook, waitFor, act } from '@testing-library/react';

// ... inside describe('useMapData') ...

it('adds uid to savedUids on save', async () => {
  const spy = new SpyMapPort();
  spy.fetchMyAuthoritiesResult = [anAuthority()];
  spy.fetchApplicationsByAuthorityResults.set(1, [anApplication()]);
  spy.fetchSavedApplicationsResult = [];

  const { result } = renderHook(() => useMapData(spy));

  await waitFor(() => {
    expect(result.current.isLoading).toBe(false);
  });

  await act(async () => {
    await result.current.saveApplication(asApplicationUid('app-001'));
  });

  expect(result.current.savedUids.has(asApplicationUid('app-001'))).toBe(true);
  expect(spy.saveApplicationCalls).toEqual([asApplicationUid('app-001')]);
});

it('removes uid from savedUids on unsave', async () => {
  const spy = new SpyMapPort();
  spy.fetchMyAuthoritiesResult = [anAuthority()];
  spy.fetchApplicationsByAuthorityResults.set(1, [anApplication()]);
  spy.fetchSavedApplicationsResult = [aSavedApplication()];

  const { result } = renderHook(() => useMapData(spy));

  await waitFor(() => {
    expect(result.current.isLoading).toBe(false);
  });

  await act(async () => {
    await result.current.unsaveApplication(asApplicationUid('app-001'));
  });

  expect(result.current.savedUids.has(asApplicationUid('app-001'))).toBe(false);
  expect(spy.unsaveApplicationCalls).toEqual([asApplicationUid('app-001')]);
});

it('reverts savedUids when save fails', async () => {
  const spy = new SpyMapPort();
  spy.fetchMyAuthoritiesResult = [anAuthority()];
  spy.fetchApplicationsByAuthorityResults.set(1, [anApplication()]);
  spy.fetchSavedApplicationsResult = [];
  spy.saveApplicationError = new Error('Server error');

  const { result } = renderHook(() => useMapData(spy));

  await waitFor(() => {
    expect(result.current.isLoading).toBe(false);
  });

  await act(async () => {
    await result.current.saveApplication(asApplicationUid('app-001'));
  });

  expect(result.current.savedUids.has(asApplicationUid('app-001'))).toBe(false);
});

it('reverts savedUids when unsave fails', async () => {
  const spy = new SpyMapPort();
  spy.fetchMyAuthoritiesResult = [anAuthority()];
  spy.fetchApplicationsByAuthorityResults.set(1, [anApplication()]);
  spy.fetchSavedApplicationsResult = [aSavedApplication()];
  spy.unsaveApplicationError = new Error('Server error');

  const { result } = renderHook(() => useMapData(spy));

  await waitFor(() => {
    expect(result.current.isLoading).toBe(false);
  });

  await act(async () => {
    await result.current.unsaveApplication(asApplicationUid('app-001'));
  });

  expect(result.current.savedUids.has(asApplicationUid('app-001'))).toBe(true);
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd web && npx vitest run src/features/Map/__tests__/useMapData.test.ts`
Expected: FAIL — `saveApplication` and `unsaveApplication` not returned

- [ ] **Step 3: Implement save/unsave with optimistic updates**

Replace `useMapData.ts` with:

```typescript
import { useState, useCallback, useMemo } from 'react';
import type { ApplicationUid, PlanningApplication } from '../../domain/types';
import type { MapPort } from '../../domain/ports/map-port';
import { useFetchData } from '../../hooks/useFetchData';

interface MapData {
  readonly applications: readonly PlanningApplication[];
  readonly fetchedSavedUids: ReadonlySet<ApplicationUid>;
}

export function useMapData(port: MapPort) {
  const { data, isLoading, error, refresh } = useFetchData<MapData>(
    async () => {
      const [authorities, savedApps] = await Promise.all([
        port.fetchMyAuthorities(),
        port.fetchSavedApplications(),
      ]);

      const uniqueAuthorityIds = [...new Set(authorities.map(a => a.id))];
      const applicationArrays = await Promise.all(
        uniqueAuthorityIds.map(id => port.fetchApplicationsByAuthority(id)),
      );

      return {
        applications: applicationArrays.flat(),
        fetchedSavedUids: new Set(savedApps.map(s => s.applicationUid)),
      };
    },
    [port],
  );

  const [pendingSaves, setPendingSaves] = useState(new Set<ApplicationUid>());
  const [pendingRemoves, setPendingRemoves] = useState(new Set<ApplicationUid>());

  const savedUids: ReadonlySet<ApplicationUid> = useMemo(() => {
    const result = new Set(data?.fetchedSavedUids ?? []);
    for (const uid of pendingSaves) result.add(uid);
    for (const uid of pendingRemoves) result.delete(uid);
    return result;
  }, [data?.fetchedSavedUids, pendingSaves, pendingRemoves]);

  const saveApplication = useCallback(async (uid: ApplicationUid) => {
    setPendingSaves(prev => new Set([...prev, uid]));
    try {
      await port.saveApplication(uid);
    } catch {
      setPendingSaves(prev => {
        const next = new Set(prev);
        next.delete(uid);
        return next;
      });
    }
  }, [port]);

  const unsaveApplication = useCallback(async (uid: ApplicationUid) => {
    setPendingRemoves(prev => new Set([...prev, uid]));
    try {
      await port.unsaveApplication(uid);
    } catch {
      setPendingRemoves(prev => {
        const next = new Set(prev);
        next.delete(uid);
        return next;
      });
    }
  }, [port]);

  return {
    applications: data?.applications ?? [],
    savedUids,
    isLoading,
    error,
    refresh,
    saveApplication,
    unsaveApplication,
  };
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd web && npx vitest run src/features/Map/__tests__/useMapData.test.ts`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add src/features/Map/__tests__/useMapData.test.ts src/features/Map/useMapData.ts
git commit -m "feat(web): add optimistic save/unsave mutations to useMapData"
```

---

### Task 5: Create Marker Icon Utilities

**Files:**
- Create: `src/features/Map/markerIcons.ts`

- [ ] **Step 1: Create SVG-based DivIcon markers**

```typescript
import L from 'leaflet';

const SAVED_COLOR = '#E9A620';
const UNSAVED_COLOR = '#94A3B8';

function pinSvg(fill: string, inner: string): string {
  return `<svg viewBox="0 0 25 41" width="25" height="41" xmlns="http://www.w3.org/2000/svg">
    <path d="M12.5 0C5.6 0 0 5.6 0 12.5C0 21.9 12.5 41 12.5 41S25 21.9 25 12.5C25 5.6 19.4 0 12.5 0Z" fill="${fill}"/>
    ${inner}
  </svg>`;
}

export const savedMarkerIcon = L.divIcon({
  html: pinSvg(SAVED_COLOR, '<path d="M9 7h7v11l-3.5-2.5L9 18V7z" fill="white"/>'),
  className: '',
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
});

export const unsavedMarkerIcon = L.divIcon({
  html: pinSvg(UNSAVED_COLOR, '<circle cx="12.5" cy="12.5" r="4.5" fill="white" fill-opacity="0.9"/>'),
  className: '',
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
});
```

- [ ] **Step 2: Verify types compile**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add src/features/Map/markerIcons.ts
git commit -m "feat(web): add SVG-based marker icons for saved/unsaved states"
```

---

### Task 6: Create FitBounds Component

**Files:**
- Create: `src/features/Map/FitBounds.tsx`

- [ ] **Step 1: Create FitBounds following FitToCircle pattern**

```typescript
import { useEffect, useRef } from 'react';
import { useMap } from 'react-leaflet';
import L from 'leaflet';

interface FitBoundsProps {
  positions: readonly [number, number][];
}

export function FitBounds({ positions }: FitBoundsProps) {
  const map = useMap();
  const hasFit = useRef(false);

  useEffect(() => {
    if (hasFit.current || positions.length === 0) return;
    const bounds = L.latLngBounds(
      positions.map(([lat, lng]) => L.latLng(lat, lng)),
    );
    map.fitBounds(bounds, { padding: [50, 50] });
    hasFit.current = true;
  }, [map, positions]);

  return null;
}
```

Note: `FitBounds` fits once on initial load, then stops. This prevents jarring viewport jumps when the user saves/unsaves applications while exploring the map.

- [ ] **Step 2: Verify types compile**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add src/features/Map/FitBounds.tsx
git commit -m "feat(web): add FitBounds component for auto-fit map viewport"
```

---

### Task 7: Create BookmarkButton Component

**Files:**
- Create: `src/features/Map/BookmarkButton.tsx`
- Create: `src/features/Map/BookmarkButton.module.css`

- [ ] **Step 1: Create BookmarkButton component**

```tsx
import styles from './BookmarkButton.module.css';

interface BookmarkButtonProps {
  isSaved: boolean;
  onToggle: () => void;
}

export function BookmarkButton({ isSaved, onToggle }: BookmarkButtonProps) {
  return (
    <button
      className={`${styles.button} ${isSaved ? styles.saved : ''}`}
      onClick={onToggle}
      aria-label={isSaved ? 'Unsave application' : 'Save application'}
      type="button"
    >
      <svg viewBox="0 0 24 24" width="18" height="18" xmlns="http://www.w3.org/2000/svg">
        {isSaved ? (
          <path d="M5 3h14v18l-7-4.5L5 21V3z" fill="currentColor" />
        ) : (
          <path d="M5 3h14v18l-7-4.5L5 21V3z" stroke="currentColor" strokeWidth="1.5" fill="none" />
        )}
      </svg>
    </button>
  );
}
```

- [ ] **Step 2: Create BookmarkButton styles**

```css
.button {
  background: none;
  border: none;
  padding: 2px;
  cursor: pointer;
  color: var(--tc-text-secondary);
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color var(--tc-duration-fast);
}

.button:hover {
  color: var(--tc-amber);
}

.saved {
  color: var(--tc-amber);
}
```

- [ ] **Step 3: Verify types compile**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add src/features/Map/BookmarkButton.tsx src/features/Map/BookmarkButton.module.css
git commit -m "feat(web): add BookmarkButton component for map popups"
```

---

### Task 8: Update MapPage and Tests

**Files:**
- Modify: `src/features/Map/MapPage.tsx`
- Modify: `src/features/Map/MapPage.module.css`
- Modify: `src/features/Map/__tests__/MapPage.test.tsx`

- [ ] **Step 1: Write failing tests for bookmark buttons and save/unsave**

Replace `MapPage.test.tsx` with:

```tsx
import { render, screen, within, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MapPage } from '../MapPage';
import { SpyMapPort } from './spies/spy-map-port';
import { anAuthority, anApplication, aSecondApplication, aSavedApplication } from './fixtures/map.fixtures';
import { asApplicationUid } from '../../../domain/types';

vi.mock('react-leaflet', () => ({
  MapContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map-container">{children}</div>
  ),
  TileLayer: () => <div data-testid="tile-layer" />,
  Marker: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map-marker">{children}</div>
  ),
  Popup: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map-popup">{children}</div>
  ),
  useMap: () => ({ fitBounds: vi.fn() }),
}));

vi.mock('leaflet', () => ({
  default: {
    divIcon: () => ({}),
    latLngBounds: () => ({}),
    latLng: (lat: number, lng: number) => ({ lat, lng }),
  },
  divIcon: () => ({}),
  latLngBounds: () => ({}),
  latLng: (lat: number, lng: number) => ({ lat, lng }),
}));

describe('MapPage', () => {
  let spy: SpyMapPort;

  beforeEach(() => {
    spy = new SpyMapPort();
  });

  it('renders map heading and container', async () => {
    spy.fetchMyAuthoritiesResult = [anAuthority()];

    render(
      <MemoryRouter>
        <MapPage port={spy} />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('map-container')).toBeInTheDocument();
    });

    expect(screen.getByRole('heading', { name: 'Map' })).toBeInTheDocument();
  });

  it('renders loading state initially', () => {
    render(
      <MemoryRouter>
        <MapPage port={spy} />
      </MemoryRouter>,
    );

    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });

  it('renders error state on failure', async () => {
    spy.fetchMyAuthoritiesError = new Error('Network unavailable');

    render(
      <MemoryRouter>
        <MapPage port={spy} />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('Network unavailable')).toBeInTheDocument();
    });
  });

  it('renders application markers with popups showing summary info', async () => {
    const auth = anAuthority();
    const app = anApplication();
    spy.fetchMyAuthoritiesResult = [auth];
    spy.fetchApplicationsByAuthorityResults.set(auth.id as number, [app]);

    render(
      <MemoryRouter>
        <MapPage port={spy} />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('map-marker')).toBeInTheDocument();
    });

    expect(screen.getByText('Erection of two-storey rear extension')).toBeInTheDocument();
    expect(screen.getByText('12 Mill Road, Cambridge')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /View details/i })).toHaveAttribute(
      'href',
      '/applications/app-001',
    );
  });

  it('skips applications without coordinates', async () => {
    const auth = anAuthority();
    const appWithCoords = anApplication();
    const appWithoutCoords = anApplication({
      uid: 'no-coords' as never,
      latitude: null,
      longitude: null,
    });
    spy.fetchMyAuthoritiesResult = [auth];
    spy.fetchApplicationsByAuthorityResults.set(auth.id as number, [
      appWithCoords,
      appWithoutCoords,
    ]);

    render(
      <MemoryRouter>
        <MapPage port={spy} />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getAllByTestId('map-marker')).toHaveLength(1);
    });
  });

  it('renders "Save application" button for unsaved apps', async () => {
    const auth = anAuthority();
    spy.fetchMyAuthoritiesResult = [auth];
    spy.fetchApplicationsByAuthorityResults.set(auth.id as number, [anApplication()]);
    spy.fetchSavedApplicationsResult = [];

    render(
      <MemoryRouter>
        <MapPage port={spy} />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Save application' })).toBeInTheDocument();
    });
  });

  it('renders "Unsave application" button for saved apps', async () => {
    const auth = anAuthority();
    spy.fetchMyAuthoritiesResult = [auth];
    spy.fetchApplicationsByAuthorityResults.set(auth.id as number, [anApplication()]);
    spy.fetchSavedApplicationsResult = [aSavedApplication()];

    render(
      <MemoryRouter>
        <MapPage port={spy} />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Unsave application' })).toBeInTheDocument();
    });
  });

  it('calls saveApplication when save button is clicked', async () => {
    const auth = anAuthority();
    spy.fetchMyAuthoritiesResult = [auth];
    spy.fetchApplicationsByAuthorityResults.set(auth.id as number, [anApplication()]);
    spy.fetchSavedApplicationsResult = [];

    render(
      <MemoryRouter>
        <MapPage port={spy} />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Save application' })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Save application' }));

    await waitFor(() => {
      expect(spy.saveApplicationCalls).toEqual([asApplicationUid('app-001')]);
    });
  });

  it('calls unsaveApplication when unsave button is clicked', async () => {
    const auth = anAuthority();
    spy.fetchMyAuthoritiesResult = [auth];
    spy.fetchApplicationsByAuthorityResults.set(auth.id as number, [anApplication()]);
    spy.fetchSavedApplicationsResult = [aSavedApplication()];

    render(
      <MemoryRouter>
        <MapPage port={spy} />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Unsave application' })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Unsave application' }));

    await waitFor(() => {
      expect(spy.unsaveApplicationCalls).toEqual([asApplicationUid('app-001')]);
    });
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd web && npx vitest run src/features/Map/__tests__/MapPage.test.tsx`
Expected: FAIL — BookmarkButton not rendered, save/unsave not wired

- [ ] **Step 3: Add popup header layout to CSS**

Add to `MapPage.module.css`:

```css
.popupHeader {
  display: flex;
  align-items: flex-start;
  gap: var(--tc-space-xs);
}
```

And add `flex: 1;` to the existing `.popupDescription` rule:

```css
.popupDescription {
  font-size: var(--tc-text-caption);
  color: var(--tc-text-primary);
  margin: 0 0 var(--tc-space-xs) 0;
  flex: 1;
}
```

- [ ] **Step 4: Update MapPage to use all new components**

Replace `MapPage.tsx` with:

```tsx
import { useMemo } from 'react';
import L from 'leaflet';
import { Link } from 'react-router-dom';
import { MapContainer, TileLayer, Marker, Popup } from 'react-leaflet';
import type { MapPort } from '../../domain/ports/map-port';
import { useMapData } from './useMapData';
import { savedMarkerIcon, unsavedMarkerIcon } from './markerIcons';
import { FitBounds } from './FitBounds';
import { BookmarkButton } from './BookmarkButton';
import styles from './MapPage.module.css';
import 'leaflet/dist/leaflet.css';

const OSM_TILE_URL = 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png';
const OSM_ATTRIBUTION =
  '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors';

const UK_CENTER: [number, number] = [51.5074, -0.1278];
const DEFAULT_ZOOM = 13;

interface Props {
  port: MapPort;
}

export function MapPage({ port }: Props) {
  const { applications, savedUids, isLoading, error, saveApplication, unsaveApplication } =
    useMapData(port);

  const markableApplications = useMemo(
    () => applications.filter(app => app.latitude !== null && app.longitude !== null),
    [applications],
  );

  const fitPositions = useMemo(() => {
    const savedApps = markableApplications.filter(app => savedUids.has(app.uid));
    const targets = savedApps.length > 0 ? savedApps : markableApplications;
    return targets.map(app => [app.latitude!, app.longitude!] as [number, number]);
  }, [markableApplications, savedUids]);

  if (isLoading) {
    return (
      <div className={styles.container}>
        <h1 className={styles.heading}>Map</h1>
        <div className={styles.loading}>Loading...</div>
      </div>
    );
  }

  if (error !== null) {
    return (
      <div className={styles.container}>
        <h1 className={styles.heading}>Map</h1>
        <div className={styles.error}>{error}</div>
      </div>
    );
  }

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Map</h1>
      <div className={styles.mapWrapper}>
        <MapContainer
          center={UK_CENTER}
          zoom={DEFAULT_ZOOM}
          style={{ height: '100%', width: '100%' }}
        >
          <TileLayer url={OSM_TILE_URL} attribution={OSM_ATTRIBUTION} />
          <FitBounds positions={fitPositions} />
          {markableApplications.map(app => {
            const isSaved = savedUids.has(app.uid);
            return (
              <Marker
                key={app.uid}
                position={[app.latitude!, app.longitude!]}
                icon={isSaved ? savedMarkerIcon : unsavedMarkerIcon}
              >
                <Popup>
                  <div className={styles.popupHeader}>
                    <p className={styles.popupDescription}>{app.description}</p>
                    <BookmarkButton
                      isSaved={isSaved}
                      onToggle={() =>
                        isSaved ? unsaveApplication(app.uid) : saveApplication(app.uid)
                      }
                    />
                  </div>
                  <p className={styles.popupAddress}>{app.address}</p>
                  <Link
                    className={styles.popupLink}
                    to={`/applications/${app.uid}`}
                  >
                    View details
                  </Link>
                </Popup>
              </Marker>
            );
          })}
        </MapContainer>
      </div>
    </div>
  );
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd web && npx vitest run src/features/Map/__tests__/MapPage.test.tsx`
Expected: All tests PASS

- [ ] **Step 6: Run full test suite**

Run: `cd web && npx vitest run`
Expected: All tests PASS

- [ ] **Step 7: Type check**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 8: Commit**

```bash
git add src/features/Map/MapPage.tsx src/features/Map/MapPage.module.css src/features/Map/__tests__/MapPage.test.tsx
git commit -m "feat(web): wire color-coded markers, bookmark buttons, and FitBounds into MapPage"
```
